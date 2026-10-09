package ops

import (
	"strings"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/imageref"
	"github.com/sirupsen/logrus"
)

// Registry image resolution for experiment workflows
// (docs/setup/registry-migration-plan.md, Phase 4).
//
// Experiment manifests, fault definitions and the hub bundle keep upstream
// public image names, so one bundle serves both networks. At submission time
// every image is resolved against IMAGE_REGISTRY / IMAGE_MIRROR_NAMESPACE
// (pkg/imageref) — but only for components whose images are pulled from the
// registry at run time. Components marked "local" in .env were side-loaded
// into the cluster under their public names, so those names must stay.
//
//	LITMUS_IMAGES_SOURCE              third-party workflow / fault images (default registry)
//	ITBENCH_EXPERIMENT_IMAGE_SOURCE   agentcert/itbench-experiment          (default local)
//	SRE_AGENTS_IMAGE_SOURCE           agent + sidecar images                (default local)
//
// ACE's own images in workflow templates (install-app / install-agent) are set
// from INSTALL_*_IMAGE, which setup.sh already resolved, and are left alone.

const itbenchExperimentRepo = "agentcert/itbench-experiment"

// workflowRuntimeFromRegistry reports whether third-party workflow images are
// pulled from the registry at run time.
func workflowRuntimeFromRegistry() bool {
	return imageref.SourceIsRegistry("LITMUS_IMAGES_SOURCE", "registry")
}

// isACEImage reports whether ref names an image in ACE's own namespace
// (built from this repo, or a frozen copy that is already resolved).
func isACEImage(ref string) bool {
	return strings.HasPrefix(imageref.Canonical(ref), imageref.DefaultMirrorNamespace+"/")
}

// workflowImageSkip returns the predicate deciding which images in a workflow
// must keep their name under the current .env sources.
func workflowImageSkip() func(string) bool {
	runtime := workflowRuntimeFromRegistry()
	itbench := imageref.SourceIsRegistry("ITBENCH_EXPERIMENT_IMAGE_SOURCE", "local")
	return func(ref string) bool {
		if isACEImage(ref) {
			return !(itbench && strings.HasPrefix(imageref.Canonical(ref), itbenchExperimentRepo+":"))
		}
		return !runtime
	}
}

// WorkflowImage resolves an image the server itself writes into a workflow
// (e.g. the litmuschaos/k8s and busybox helper steps added by the run patches).
func WorkflowImage(ref string) string {
	r := imageref.FromEnv()
	if !r.Active() || workflowImageSkip()(ref) {
		return ref
	}
	return r.Resolve(ref)
}

// ApplyRegistryImageOverrides resolves every image in a workflow spec — template
// containers, init containers, sidecars, script steps, and the fault/engine
// definitions embedded as raw artifacts (their "image:" lines and *_IMAGE
// settings, except the intentionally broken INVALID_IMAGE / INVALID_ARCH_IMAGE)
// — and attaches the image-pull Secret when a private registry is configured.
// Idempotent; a no-op on the open-source path with IMAGE_MIRROR_NAMESPACE=none.
func ApplyRegistryImageOverrides(spec *v1alpha1.WorkflowSpec) {
	if spec == nil {
		return
	}
	changed := 0
	if secret := imageref.PullSecretName(); secret != "" {
		found := false
		for _, s := range spec.ImagePullSecrets {
			if s.Name == secret {
				found = true
				break
			}
		}
		if !found {
			spec.ImagePullSecrets = append(spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
			changed++
		}
	}

	r := imageref.FromEnv()
	if r.Active() {
		skip := workflowImageSkip()
		resolve := func(img *string) {
			if *img == "" || skip(*img) {
				return
			}
			if n := r.Resolve(*img); n != *img {
				*img = n
				changed++
			}
		}
		for i := range spec.Templates {
			t := &spec.Templates[i]
			if t.Container != nil {
				resolve(&t.Container.Image)
			}
			if t.Script != nil {
				resolve(&t.Script.Image)
			}
			for j := range t.InitContainers {
				resolve(&t.InitContainers[j].Image)
			}
			for j := range t.Sidecars {
				resolve(&t.Sidecars[j].Image)
			}
			for j := range t.Inputs.Artifacts {
				raw := t.Inputs.Artifacts[j].Raw
				if raw == nil || raw.Data == "" {
					continue
				}
				if n := r.RewriteText(raw.Data, skip); n != raw.Data {
					raw.Data = n
					changed++
				}
			}
		}
	}

	if changed > 0 {
		logrus.WithFields(logrus.Fields{
			"registry":         r.Registry,
			"mirror_namespace": r.MirrorNamespace,
			"changes":          changed,
		}).Info("[Registry Images] Resolved workflow images against IMAGE_REGISTRY")
	}
}

// installerImageEnv returns the post-renderer settings for an install-app /
// install-agent step whose charts' images come from the registry at run time
// (nil when they were side-loaded locally, so the charts keep public names).
func installerImageEnv(sourceKey, def string) []corev1.EnvVar {
	r := imageref.FromEnv()
	if !imageref.SourceIsRegistry(sourceKey, def) || !r.Active() {
		return nil
	}
	return []corev1.EnvVar{
		{Name: "ACE_IMAGE_REGISTRY", Value: r.Registry},
		{Name: "ACE_IMAGE_MIRROR_NAMESPACE", Value: r.MirrorNamespace},
		{Name: "ACE_IMAGE_TAG", Value: r.AceTag},
		{Name: "ACE_IMAGE_PULL_SECRET", Value: imageref.PullSecretName()},
	}
}

// setContainerEnv sets (or replaces) env vars on an Argo template container.
// Older installer images ignore variables they do not know, so this is safe
// for any installer version.
func setContainerEnv(c *corev1.Container, vars []corev1.EnvVar) bool {
	changed := false
	for _, v := range vars {
		replaced := false
		for i := range c.Env {
			if c.Env[i].Name == v.Name {
				if c.Env[i].Value != v.Value {
					c.Env[i].Value = v.Value
					changed = true
				}
				replaced = true
				break
			}
		}
		if !replaced {
			c.Env = append(c.Env, v)
			changed = true
		}
	}
	return changed
}

// splitChartImage splits an image reference into the registry / repository /
// tag triple the agent charts render as "<registry>/<repository>:<tag>".
// The registry is the reference's own host, or docker.io when it has none.
//
//	agentcert/agent-sidecar:latest -> docker.io, agentcert/agent-sidecar, latest
//	reg.io/path/agentcert/x:v1     -> reg.io, path/agentcert/x, v1
func splitChartImage(ref string) (registry, repository, tag string) {
	ref = strings.TrimSpace(ref)
	tag = "latest"
	last := strings.LastIndex(ref, "/")
	if i := strings.LastIndex(ref, ":"); i > last {
		ref, tag = ref[:i], ref[i+1:]
	}
	registry = "docker.io"
	if i := strings.Index(ref, "/"); i > 0 && (strings.ContainsAny(ref[:i], ".:") || ref[:i] == "localhost") {
		registry, ref = ref[:i], ref[i+1:]
	}
	return registry, ref, tag
}
