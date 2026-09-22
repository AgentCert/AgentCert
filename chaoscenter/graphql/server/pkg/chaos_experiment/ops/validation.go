package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/agenthub"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/faultcatalog"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Composition errors are returned to the caller as-is, so they have to read as
// instructions rather than as internal diagnostics.
const (
	errNoInstallApplication = "this experiment %s but installs no application. " +
		"Add a target application first: the agent is installed into its namespace, the faults target its " +
		"workloads, and the generated workflow cannot resolve {{workflow.parameters.appNamespace}} without it"
	errMultipleInstallApplications = "found %d install-application steps (%s). An experiment targets exactly one " +
		"application, because appNamespace is a single workflow parameter"
	errNoAppNamespace = "the install-application step declares no -namespace=. " +
		"Without it {{workflow.parameters.appNamespace}} cannot be resolved and Argo rejects the workflow spec"
	errNoAgentFolder = "this experiment has an %s step but no resolvable -folder=. " +
		"Without it {{workflow.parameters.agentFolder}} cannot be resolved and Argo rejects the workflow spec"
	errIncompatibleFault = "fault %q cannot target application %q. It is compatible with: %s"
	errIncompatibleAgent = "agent %q cannot be paired with application %q"
	errAgentNotAppScoped = "agent %q is not application-targeted and cannot be used with application %q"
)

// appNamespaceRef is emitted unconditionally by applyInstallApplicationReadinessPatch,
// applyUninstallAllPatch and InjectExperimentContextArgs.
const appNamespaceRef = "{{workflow.parameters.appNamespace}}"

// installStepArg reads a flag from a template's container args, accepting both
// the combined (-flag=value) and split (-flag value) forms.
func installStepArg(args []string, flag string) string {
	for i, arg := range args {
		for _, prefix := range []string{"-" + flag + "=", "--" + flag + "="} {
			if strings.HasPrefix(arg, prefix) {
				return strings.TrimPrefix(arg, prefix)
			}
		}
		if (arg == "-"+flag || arg == "--"+flag) && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// isInstallStepTemplate identifies an install step by annotation, name, or image.
//
// The image alone is not sufficient: teardown steps reuse the very same image
// (the stock `delete-application` template runs agentcert-install-app with
// `-operation=delete`), so matching on it alone counts one application twice and
// rejects a perfectly valid workflow. Operation and name are checked first.
func isInstallStepTemplate(t v1alpha1.Template, kind string) bool {
	if t.Container == nil {
		return false
	}
	if installType, ok := t.Metadata.Annotations["agentcert.io/install-type"]; ok {
		return installType == kind
	}
	if op := installStepArg(t.Container.Args, "operation"); op != "" && !strings.EqualFold(op, "apply") {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(t.Name))
	if strings.HasPrefix(name, "uninstall") || strings.HasPrefix(name, "delete") || strings.HasPrefix(name, "cleanup") {
		return false
	}

	legacyName, imageMarker := "install-application", "agentcert-install-app"
	if kind == "agent" {
		legacyName, imageMarker = "install-agent", "agentcert-install-agent"
	}
	return name == legacyName || strings.Contains(strings.TrimSpace(t.Container.Image), imageMarker)
}

// chaosFaultNames returns the faults a workflow injects, read from the
// ChaosEngine artifacts staged by the install-chaos-faults step. That is the
// same source the builder counts, so the two cannot disagree about whether an
// experiment "has faults".
func chaosFaultNames(templates []v1alpha1.Template) []string {
	var faults []string
	for _, t := range templates {
		if t.Name != "install-chaos-faults" && t.Name != "install-chaos-experiments" {
			continue
		}
		if t.Inputs.Artifacts == nil {
			continue
		}
		for _, artifact := range t.Inputs.Artifacts {
			if artifact.Raw == nil {
				continue
			}
			var engine struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Name string `yaml:"name"`
				} `yaml:"metadata"`
				Spec struct {
					Experiments []struct {
						Name string `yaml:"name"`
					} `yaml:"experiments"`
				} `yaml:"spec"`
			}
			if err := yaml.Unmarshal([]byte(artifact.Raw.Data), &engine); err != nil {
				continue
			}
			if !strings.EqualFold(engine.Kind, "ChaosEngine") {
				// install-chaos-faults also stages the ChaosExperiment CRs; the
				// engine is what names the fault actually being run.
				continue
			}
			for _, exp := range engine.Spec.Experiments {
				if exp.Name != "" {
					faults = append(faults, exp.Name)
				}
			}
		}
	}
	return faults
}

// agentCompatibility returns an agent chart's declared application restriction.
// The second result is false when the agent is unknown to the hub, which must
// not be treated as a restriction — a custom agent hub is still valid.
func agentCompatibility(agentFolder string) ([]string, bool) {
	entries, err := agenthub.GetAllAgentEntries(agenthub.ChartsPath())
	if err != nil {
		log.WithError(err).Warn("[Validation] could not read agent charts; skipping agent compatibility check")
		return nil, false
	}
	for _, entry := range entries {
		if entry.Name != agentFolder {
			continue
		}
		if entry.CompatibleApplications == nil {
			return nil, false
		}
		return *entry.CompatibleApplications, true
	}
	return nil, false
}

// validateExperimentComposition is the authoritative gate on the Save/Run path.
//
// It exists because the builder used to allow a manifest shape the server's own
// patches assume cannot occur: applyInstallApplicationReadinessPatch and
// applyUninstallAllPatch emit {{workflow.parameters.appNamespace}} and
// {{workflow.parameters.agentFolder}} unconditionally, but a missing one only
// produced a log warning and the workflow was accepted, then rejected by Argo's
// spec validation before a single step ran. Rejecting it here turns that into an
// error the user can act on.
//
// The application requirement is deliberately scoped to the experiments that
// actually need one — those installing an ACE agent, or referencing appNamespace
// — rather than to every experiment with a fault. A stock LitmusChaos workflow
// imported from a hub has neither and must keep working; the builder's own
// app-first gating is what enforces the ordering for newly authored ones.
//
// Every compatibility check fails OPEN against unknown inputs: an uncatalogued
// fault, an unregistered application, or an agent from a custom hub is allowed
// through, so onboarding any of the three never requires a code change.
func (c *chaosExperimentService) validateExperimentComposition(
	ctx context.Context,
	projectID string,
	workflowSpec *v1alpha1.WorkflowSpec,
) error {
	if workflowSpec == nil {
		return nil
	}
	templates := workflowSpec.Templates

	var appFolders, appNamespaces []string
	for _, t := range templates {
		if !isInstallStepTemplate(t, "application") {
			continue
		}
		appFolders = append(appFolders, installStepArg(t.Container.Args, "folder"))
		appNamespaces = append(appNamespaces, installStepArg(t.Container.Args, "namespace"))
	}

	if len(appFolders) > 1 {
		return fmt.Errorf(errMultipleInstallApplications, len(appFolders), strings.Join(appFolders, ", "))
	}

	hasInstallAgent := false
	hasUninstallAgent := false
	for _, t := range templates {
		if isInstallStepTemplate(t, "agent") {
			hasInstallAgent = true
		}
		if strings.EqualFold(strings.TrimSpace(t.Name), "uninstall-agent") {
			hasUninstallAgent = true
		}
	}

	// An experiment needs an application only when something in it will resolve
	// against one.
	needsApplication := hasInstallAgent || referencesAppNamespace(workflowSpec)
	if len(appFolders) == 0 {
		if !needsApplication {
			return nil
		}
		reason := "references " + appNamespaceRef
		if hasInstallAgent {
			reason = "installs an agent"
		}
		return fmt.Errorf(errNoInstallApplication, reason)
	}
	if strings.TrimSpace(appNamespaces[0]) == "" {
		return errors.New(errNoAppNamespace)
	}

	// agentFolder is referenced by the generated uninstall-all script, which is
	// injected whenever an install-agent step exists.
	if (hasInstallAgent || hasUninstallAgent) && ExtractInstallAgentFolder(templates) == "" {
		step := "install-agent"
		if !hasInstallAgent {
			step = "uninstall-agent"
		}
		return fmt.Errorf(errNoAgentFolder, step)
	}

	if c.faultCatalogService == nil {
		return nil
	}
	catalog, err := c.faultCatalogService.Catalog(ctx, projectID)
	if err != nil {
		// The catalogue is advisory for compatibility; the structural checks
		// above are what prevent the Argo rejection, and those already passed.
		log.WithError(err).Warn("[Validation] fault catalogue unavailable; skipping compatibility checks")
		return nil
	}

	app, known := catalog.ResolveApplication(appFolders[0], appNamespaces[0])
	if !known {
		return nil // an application the registry has not heard of restricts nothing
	}

	for _, fault := range chaosFaultNames(templates) {
		if catalog.IsFaultCompatible(fault, app.Key) {
			continue
		}
		compatible, _ := catalog.Lookup(fault)
		allowed := "no application"
		if len(compatible.CompatibleApps) > 0 {
			sorted := append([]string(nil), compatible.CompatibleApps...)
			sort.Strings(sorted)
			allowed = strings.Join(sorted, ", ")
		}
		return fmt.Errorf(errIncompatibleFault, fault, app.Key, allowed)
	}

	if agentFolder := ExtractInstallAgentFolder(templates); agentFolder != "" {
		if declared, restricted := agentCompatibility(agentFolder); restricted {
			if len(declared) == 0 {
				return fmt.Errorf(errAgentNotAppScoped, agentFolder, app.Key)
			}
			if !faultcatalog.IsAgentCompatible(declared, app.Key) {
				return fmt.Errorf(errIncompatibleAgent, agentFolder, app.Key)
			}
		}
	}

	return nil
}

// referencesAppNamespace reports whether anything in the spec interpolates
// {{workflow.parameters.appNamespace}}, in which case the parameter must be
// resolvable or Argo rejects the whole workflow.
func referencesAppNamespace(spec *v1alpha1.WorkflowSpec) bool {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return false
	}
	return strings.Contains(string(encoded), appNamespaceRef)
}
