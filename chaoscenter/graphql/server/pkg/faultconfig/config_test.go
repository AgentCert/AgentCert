package faultconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp/syntax"
	"strings"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/ghodss/yaml"
	chaosTypes "github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chartconfig"
	corev1 "k8s.io/api/core/v1"
)

// sampleForPattern returns a value satisfying pattern, so a pattern-constrained
// string field can be probed with something other than its default. It walks the
// parsed expression taking the first branch of every alternation and skipping
// repeats, which yields a concrete match for the literal-alternative patterns the
// catalog uses. Returns "" when no literal can be derived; every pattern in the
// catalog admits the empty string, so that stays a valid probe.
func sampleForPattern(pattern string) string {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return ""
	}
	var first func(*syntax.Regexp) string
	first = func(r *syntax.Regexp) string {
		switch r.Op {
		case syntax.OpLiteral:
			return string(r.Rune)
		case syntax.OpCharClass:
			// The parser factors common prefixes, so "(200|201|202|204)"
			// becomes "20" followed by the class [0124]. Rune holds inclusive
			// lo/hi pairs; the first lo is always a member.
			if len(r.Rune) > 0 {
				return string(r.Rune[0])
			}
			return ""
		case syntax.OpCapture, syntax.OpQuest, syntax.OpPlus:
			return first(r.Sub[0])
		case syntax.OpAlternate:
			return first(r.Sub[0])
		case syntax.OpConcat:
			var builder strings.Builder
			for _, sub := range r.Sub {
				builder.WriteString(first(sub))
			}
			return builder.String()
		}
		// Anchors and repeats contribute nothing.
		return ""
	}
	return first(parsed)
}

const testCSV = `spec:
  configurations:
    - key: BROKEN_TARGET_PORT
      label: Broken port
      type: integer
      required: true
      min: 1
      max: 65535
    - key: RAMP_TIME
      type: integer
      min: 0
`

const testFault = `kind: ChaosExperiment
metadata:
  name: service-port
spec:
  definition:
    env:
      - name: BROKEN_TARGET_PORT
        value: "9999"
      - name: RAMP_TIME
        value: ""
      - name: CREDENTIAL
        valueFrom:
          secretKeyRef:
            name: credentials
            key: password
`

func TestConfigurationValidation(t *testing.T) {
	definition, err := Parse([]byte(testCSV), []byte(testFault))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		env  []corev1.EnvVar
		fail bool
	}{
		{"defaults", nil, false},
		{"changed port", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", Value: "4321"}}, false},
		{"optional empty", []corev1.EnvVar{{Name: "RAMP_TIME", Value: ""}}, false},
		{"secret retained", []corev1.EnvVar{definition.Defaults["CREDENTIAL"]}, false},
		{"low", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", Value: "0"}}, true},
		{"high", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", Value: "65536"}}, true},
		{"fraction", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", Value: "123.5"}}, true},
		{"empty required", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", Value: ""}}, true},
		{"template expression", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", Value: "{{workflow.parameters.port}}"}}, true},
		{"unknown", []corev1.EnvVar{{Name: "UNKNOWN", Value: "x"}}, true},
		{"hidden changed", []corev1.EnvVar{{Name: "CREDENTIAL", Value: "plain text"}}, true},
		{"reference for a numeric field", []corev1.EnvVar{{Name: "BROKEN_TARGET_PORT", ValueFrom: definition.Defaults["CREDENTIAL"].ValueFrom}}, true},
		{"duplicate", []corev1.EnvVar{{Name: "RAMP_TIME", Value: "1"}, {Name: "RAMP_TIME", Value: "2"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateEnvironment(definition, tc.env); (err != nil) != tc.fail {
				t.Fatalf("error = %v, want failure %v", err, tc.fail)
			}
		})
	}
	for _, block := range []string{
		"spec: {configurations: null}",
		"spec: {configurations: [{key: MISSING, type: string}]}",
		"spec: {configurations: [{key: BROKEN_TARGET_PORT, type: integer, requried: true}]}",
		"spec: {configurations: [{key: BROKEN_TARGET_PORT, type: integer, min: 10000}]}",
		"spec: {configurations: [{key: BROKEN_TARGET_PORT, type: string}, {key: BROKEN_TARGET_PORT, type: string}]}",
	} {
		if _, err := Parse([]byte(block), []byte(testFault)); err == nil {
			t.Fatalf("accepted malformed metadata: %s", block)
		}
	}
	if _, err := Parse([]byte(strings.ReplaceAll(testCSV, "BROKEN_TARGET_PORT", "TARGETS")), []byte(strings.ReplaceAll(testFault, "BROKEN_TARGET_PORT", "TARGETS"))); err == nil {
		t.Fatal("TARGETS must remain platform-owned")
	}
	legacy, err := Parse([]byte("spec: {}"), []byte(testFault))
	if err != nil || legacy.HasSchema {
		t.Fatalf("legacy fault: %#v, %v", legacy, err)
	}
}

func TestCacheReloadsChangedContent(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "service-port.chartserviceversion.yaml")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fault.yaml"), []byte(testFault), 0600); err != nil {
		t.Fatal(err)
	}
	cache := NewCache()
	first, err := cache.Load(dir, "service-port")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(csvPath)
	changed := strings.Replace(testCSV, "Broken port", "Edited port", 1)
	if err := os.WriteFile(csvPath, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(csvPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	second, err := cache.Load(dir, "service-port")
	if err != nil || second.Revision == first.Revision || second.Fields[0].Label != "Edited port" {
		t.Fatalf("did not reload metadata: %#v, %v", second, err)
	}
	if first.Fields[0].Label != "Broken port" {
		t.Fatal("changed a previously resolved definition")
	}
}

func TestWorkflowUsesCanonicalSchemaAndRevision(t *testing.T) {
	definition, err := Parse([]byte(testCSV), []byte(testFault))
	if err != nil {
		t.Fatal(err)
	}
	definition.Revision = strings.Repeat("a", 64)
	source := Source{HubID: "hub", Category: "itbench", Name: "service-port", Revision: definition.Revision}
	resolve := func(_ context.Context, ref Source, project string) (*Definition, error) {
		if project != "project" || ref.Name != source.Name {
			t.Fatal("incorrect project or fault identity")
		}
		return definition, nil
	}
	for _, tc := range []struct {
		name, port, revision string
		fail                 bool
	}{
		{"valid", "4321", definition.Revision, false},
		{"invalid", "70000", definition.Revision, true},
		{"stale", "4321", strings.Repeat("b", 64), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source.Revision = tc.revision
			rawSource, _ := json.Marshal(source)
			engine := fmt.Sprintf(`kind: ChaosEngine
metadata:
  annotations:
    %s: '%s'
spec:
  experiments:
    - name: service-port
      spec:
        components:
          env:
            - name: BROKEN_TARGET_PORT
              value: '%s'
`, SourceAnnotation, rawSource, tc.port)
			templates := []v1alpha1.Template{{Name: "inject", Inputs: v1alpha1.Inputs{Artifacts: v1alpha1.Artifacts{{ArtifactLocation: v1alpha1.ArtifactLocation{Raw: &v1alpha1.RawArtifact{Data: engine}}}}}}}
			if err := ValidateWorkflow(context.Background(), "project", &v1alpha1.WorkflowSpec{Templates: templates}, resolve); (err != nil) != tc.fail {
				t.Fatalf("error = %v, want failure %v", err, tc.fail)
			}
			if err := ValidateWorkflow(context.Background(), "project", &v1alpha1.WorkflowSpec{Templates: templates}, nil); err == nil {
				t.Fatal("configured fault bypassed validation without a provider")
			}
		})
	}
}

func TestAllCatalogSettingsReachEngineEnvironment(t *testing.T) {
	root := "../../../../../../chaos-charts/faults"
	cache := NewCache()
	count, fieldCount := 0, 0
	for _, category := range []string{"kubernetes", "itbench"} {
		files, err := filepath.Glob(filepath.Join(root, category, "*", "fault.yaml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("catalog %s unavailable: %v", category, err)
		}
		for _, file := range files {
			name := filepath.Base(filepath.Dir(file))
			if strings.HasPrefix(name, "install-") || strings.HasPrefix(name, "uninstall-") {
				continue
			}
			definition, err := cache.Load(filepath.Dir(file), name)
			if err != nil {
				t.Fatalf("%s/%s: %v", category, name, err)
			}
			if !definition.HasSchema || len(definition.Fields) == 0 {
				t.Fatalf("%s/%s has no configuration metadata", category, name)
			}
			count++
			for _, field := range definition.Fields {
				value := "fault-config-probe"
				switch field.Type {
				case chartconfig.TypeSelect:
					value = field.Options[0]
					if value == field.FormatDefault() && len(field.Options) > 1 {
						value = field.Options[1]
					}
				case chartconfig.TypeBoolean:
					value = fmt.Sprint(field.FormatDefault() != "true")
				case chartconfig.TypeInteger, chartconfig.TypeNumber:
					value = "1"
					if field.Min != nil {
						value = fmt.Sprint(*field.Min)
					}
				case chartconfig.TypeString, chartconfig.TypeText:
					// A pattern-constrained field rejects the generic probe.
					if field.Pattern != "" {
						value = sampleForPattern(field.Pattern)
					}
				}
				envs := []corev1.EnvVar{{Name: field.Key, Value: value}}
				encoded, _ := yaml.Marshal(envs)
				var decoded []corev1.EnvVar
				if err := yaml.Unmarshal(encoded, &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded[0].Value != value {
					t.Fatalf("%s lost %s during serialization", name, field.Key)
				}
				if err := ValidateEnvironment(definition, decoded); err != nil {
					t.Fatalf("%s/%s field %s: %v", category, name, field.Key, err)
				}
				fieldCount++
			}
		}
	}
	t.Logf("validated %d faults and %d configurable fields", count, fieldCount)
}

func TestDirectoryRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "itbench"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "itbench", "escaped")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escaped", "escaped", "/etc"} {
		if _, err := Directory(root, "itbench", name); err == nil {
			t.Fatalf("accepted escaped directory %s", name)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "payload.yaml"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "payload.yaml"), filepath.Join(root, "engine.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "engine.yaml"); err == nil {
		t.Fatal("accepted an escaped file symlink")
	}
}

func TestLegacyParametersAndConfiguredDuration(t *testing.T) {
	definition, err := Parse([]byte(testCSV), []byte(testFault))
	if err != nil {
		t.Fatal(err)
	}
	engine := `kind: ChaosEngine
spec:
  experiments:
    - name: service-port
      spec:
        components:
          env:
            - name: BROKEN_TARGET_PORT
              value: '{{workflow.parameters.port}}'
`
	resolve := func(context.Context, Source, string) (*Definition, error) { return definition, nil }
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{{Name: "inject", Inputs: v1alpha1.Inputs{Artifacts: v1alpha1.Artifacts{{ArtifactLocation: v1alpha1.ArtifactLocation{Raw: &v1alpha1.RawArtifact{Data: engine}}}}}}},
		Arguments: v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{{Name: "port", Value: v1alpha1.AnyStringPtr("4321")}}},
	}
	if err := ValidateWorkflow(context.Background(), "project", spec, resolve); err != nil {
		t.Fatal(err)
	}
	if spec.Templates[0].Inputs.Artifacts[0].Raw.Data != engine {
		t.Fatal("validation mutated the workflow")
	}
	spec.Arguments.Parameters[0].Value = v1alpha1.AnyStringPtr("70000")
	if err := ValidateWorkflow(context.Background(), "project", spec, resolve); err == nil {
		t.Fatal("invalid parameter value bypassed validation")
	}
	for _, configured := range []bool{false, true} {
		var engine chaosTypes.ChaosEngine
		if err := yaml.Unmarshal([]byte(`kind: ChaosEngine
metadata:
  annotations:
    litmuschaos.io/injectionType: ON_DEMAND
spec:
  experiments:
    - name: service-port
      spec:
        components:
          env:
            - name: TOTAL_CHAOS_DURATION
              value: '45'
`), &engine); err != nil {
			t.Fatal(err)
		}
		if configured {
			engine.Annotations[SourceAnnotation] = "validated-source"
		}
		ApplyLegacyOnDemandDuration(&engine)
		want := "30"
		if configured {
			want = "45"
		}
		if got := engine.Spec.Experiments[0].Spec.Components.ENV[0].Value; got != want {
			t.Fatalf("configured=%v: got duration %s, want %s", configured, got, want)
		}
	}
}

func TestUnrelatedRawArtifactsAreIgnored(t *testing.T) {
	spec := &v1alpha1.WorkflowSpec{
		Templates: []v1alpha1.Template{{Name: "script", Inputs: v1alpha1.Inputs{Artifacts: v1alpha1.Artifacts{{ArtifactLocation: v1alpha1.ArtifactLocation{Raw: &v1alpha1.RawArtifact{Data: "echo hello"}}}}}}},
	}
	if err := ValidateWorkflow(context.Background(), "project", spec, nil); err != nil {
		t.Fatal(err)
	}
}
