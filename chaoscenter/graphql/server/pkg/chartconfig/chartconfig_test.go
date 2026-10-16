package chartconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const agentValues = `
agent:
  config:
    SCAN_INTERVAL: "60"
    MODEL_ALIAS: "gpt-4o"
    LOG_LEVEL: "INFO"
    GOAL: "Diagnose faults."
    TEMPERATURE: "0.2"
    MAX_COMPLETION_TOKENS: "2048"
replicas: 1
monitoring:
  enabled: true
configurations:
  - key: agent.config.SCAN_INTERVAL
    label: Scan interval (seconds)
    type: integer
    min: 1
    group: Behaviour
  - key: agent.config.MODEL_ALIAS
    type: model
  - key: agent.config.LOG_LEVEL
    type: select
    options: [DEBUG, INFO, WARNING]
    advanced: true
  - key: agent.config.GOAL
    type: text
    required: true
  - key: agent.config.TEMPERATURE
    type: number
    min: 0
    max: 2
  - key: agent.config.MAX_COMPLETION_TOKENS
    type: integer
  - key: replicas
    type: integer
    min: 1
    max: 5
  - key: monitoring.enabled
    type: boolean
`

func mustParse(t *testing.T, values string) []Field {
	t.Helper()
	fields, err := Parse([]byte(values))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return fields
}

func TestParseTakesDefaultsAndKindsFromTheChartValues(t *testing.T) {
	fields := mustParse(t, agentValues)
	byKey := map[string]Field{}
	for _, f := range fields {
		byKey[f.Key] = f
	}

	cases := []struct {
		key         string
		kind        ValueKind
		defaultText string
	}{
		{"agent.config.SCAN_INTERVAL", KindString, "60"},
		{"replicas", KindInteger, "1"},
		{"monitoring.enabled", KindBoolean, "true"},
		{"agent.config.TEMPERATURE", KindString, "0.2"},
	}
	for _, tc := range cases {
		f, ok := byKey[tc.key]
		if !ok {
			t.Fatalf("field %s missing", tc.key)
		}
		if f.Kind != tc.kind || f.FormatDefault() != tc.defaultText {
			t.Errorf("%s: kind=%s default=%q, want kind=%s default=%q", tc.key, f.Kind, f.FormatDefault(), tc.kind, tc.defaultText)
		}
	}
	if got := byKey["agent.config.LOG_LEVEL"].Label; got != "LOG_LEVEL" {
		t.Errorf("label defaults to the last path segment, got %q", got)
	}
	if len(fields) != 8 || fields[0].Key != "agent.config.SCAN_INTERVAL" {
		t.Errorf("fields must keep declaration order, got %d fields starting with %s", len(fields), fields[0].Key)
	}
}

func TestParseWithoutConfigurationsOffersNothing(t *testing.T) {
	if fields := mustParse(t, "agent:\n  config: {}\n"); len(fields) != 0 {
		t.Fatalf("got %d fields, want none", len(fields))
	}
}

func TestParseRejectsInvalidDeclarations(t *testing.T) {
	const base = "agent:\n  config:\n    X: \"60\"\n    MODE: observe\n  secret:\n    OPENAI_API_KEY: k\n  list: [a]\nreplicas: 1\n"
	cases := []struct {
		name, decl, want string
	}{
		{"misspelt attribute", "- key: agent.config.X\n  type: integer\n  requried: true", "requried"},
		{"missing type", "- key: agent.config.X", "type is required"},
		{"unknown type", "- key: agent.config.X\n  type: color", "unknown type"},
		{"key not in values", "- key: agent.config.Y\n  type: string", "does not exist in values.yaml"},
		{"non-scalar", "- key: agent.list\n  type: string", "single value"},
		{"empty path segment", "- key: agent..X\n  type: string", "dotted values path"},
		{"platform-owned key", "- key: agent.config.AGENT_MODE\n  type: string", "set by the platform"},
		{"installer namespace key", "- key: namespaces.bookInfo\n  type: string", "set by the platform"},
		{"secret", "- key: agent.secret.OPENAI_API_KEY\n  type: string", "secrets cannot be offered"},
		{"duplicate", "- key: agent.config.X\n  type: string\n- key: agent.config.X\n  type: string", "more than once"},
		{"select without options", "- key: agent.config.MODE\n  type: select", "needs options"},
		{"default not an option", "- key: agent.config.MODE\n  type: select\n  options: [active]", "must be one of"},
		{"options on non-select", "- key: agent.config.MODE\n  type: string\n  options: [observe]", "only allowed for type select"},
		{"min above max", "- key: replicas\n  type: integer\n  min: 5\n  max: 2", "min is greater than max"},
		{"non-finite bound", "- key: replicas\n  type: integer\n  min: .nan", "finite numbers"},
		{"default out of range", "- key: replicas\n  type: integer\n  min: 2", "at least 2"},
		{"range on string", "- key: agent.config.MODE\n  type: string\n  min: 1", "min and max are only allowed"},
		{"integer type, text default", "- key: agent.config.MODE\n  type: integer", "whole number"},
		{"string type, number default", "- key: replicas\n  type: string", "needs a string default"},
		{"bad pattern", "- key: agent.config.MODE\n  type: string\n  pattern: \"[\"", "does not compile"},
		{"default fails pattern", "- key: agent.config.MODE\n  type: string\n  pattern: \"^[0-9]+$\"", "must match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(base + "configurations:\n" + indent(tc.decl)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse() error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestModelAliasIsTheOnlyUserSettablePlatformKey(t *testing.T) {
	if !IsPlatformOwned(ModelAliasKey) || !IsUserSettable(ModelAliasKey) {
		t.Fatal("MODEL_ALIAS must be platform-owned and user-settable")
	}
	if IsUserSettable("agent.config.AGENT_MODE") {
		t.Fatal("AGENT_MODE must not be user-settable")
	}
}

func TestIsSecretKeyMatchesWholeWordsOnly(t *testing.T) {
	secret := []string{"agent.secret.ANYTHING", "agent.config.OPENAI_API_KEY", "db.password", "auth.token", "x.LANGFUSE_SECRET_KEY", "x.apikey"}
	notSecret := []string{"agent.config.MAX_COMPLETION_TOKENS", "agent.config.SCAN_INTERVAL", "sockShop.frontEnd.replicas", "x.tokenizer"}
	for _, key := range secret {
		if !IsSecretKey(key) {
			t.Errorf("IsSecretKey(%q) = false, want true", key)
		}
	}
	for _, key := range notSecret {
		if IsSecretKey(key) {
			t.Errorf("IsSecretKey(%q) = true, want false", key)
		}
	}
}

func TestValidateAcceptsDeclaredValues(t *testing.T) {
	fields := mustParse(t, agentValues)
	values, err := ParseValues(`{"agent":{"config":{"SCAN_INTERVAL":"90","MODEL_ALIAS":"qwen2.5-7b","LOG_LEVEL":"DEBUG","TEMPERATURE":"1.5"}},"replicas":3,"monitoring":{"enabled":false}}`)
	if err != nil {
		t.Fatalf("ParseValues() error = %v", err)
	}
	if err := Validate(fields, values); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	fields := mustParse(t, agentValues)
	cases := []struct {
		name, values, want string
	}{
		{"undeclared key", `{"agent":{"config":{"AGENT_MODE":"active"}}}`, `"agent.config.AGENT_MODE" is not a setting this chart offers`},
		{"image override", `{"agent":{"containerImage":{"tag":"evil"}}}`, "is not a setting this chart offers"},
		{"string written as number", `{"agent":{"config":{"SCAN_INTERVAL":90}}}`, "JSON string"},
		{"number written as string", `{"replicas":"3"}`, "JSON number"},
		{"not a whole number", `{"agent":{"config":{"SCAN_INTERVAL":"1.5"}}}`, "whole number"},
		{"below min", `{"agent":{"config":{"SCAN_INTERVAL":"0"}}}`, "at least 1"},
		{"above max", `{"replicas":9}`, "at most 5"},
		{"fractional integer", `{"replicas":2.5}`, "whole number"},
		{"not a number", `{"agent":{"config":{"TEMPERATURE":"warm"}}}`, "must be a number"},
		{"NaN", `{"agent":{"config":{"TEMPERATURE":"NaN"}}}`, "must be a number"},
		{"infinity", `{"agent":{"config":{"TEMPERATURE":"Inf"}}}`, "must be a number"},
		{"dotted JSON key", `{"agent.config.SCAN_INTERVAL":"90"}`, "single path segment"},
		{"empty nested object", `{"agent":{"config":{}}}`, "empty object"},
		{"not an option", `{"agent":{"config":{"LOG_LEVEL":"TRACE"}}}`, "must be one of"},
		{"boolean as text", `{"monitoring":{"enabled":"false"}}`, "JSON boolean"},
		{"empty model", `{"agent":{"config":{"MODEL_ALIAS":""}}}`, "platform default"},
		{"required left empty", `{"agent":{"config":{"GOAL":"  "}}}`, "is required"},
		{"argo expression", `{"agent":{"config":{"GOAL":"{{workflow.uid}}"}}}`, `must not contain "{{"`},
		{"list value", `{"agent":{"config":{"GOAL":["a"]}}}`, "is a list"},
		{"null value", `{"replicas":null}`, "has no value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values, err := ParseValues(tc.values)
			if err != nil {
				t.Fatalf("ParseValues() error = %v", err)
			}
			err = Validate(fields, values)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateReportsEveryBadValue(t *testing.T) {
	fields := mustParse(t, agentValues)
	values, _ := ParseValues(`{"replicas":0,"agent":{"config":{"LOG_LEVEL":"TRACE"}}}`)
	err := Validate(fields, values)
	if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") || !strings.Contains(err.Error(), "replicas") {
		t.Fatalf("Validate() error = %v, want both bad values reported", err)
	}
}

func TestParseValuesRequiresOneJSONObject(t *testing.T) {
	for _, raw := range []string{``, `[]`, `"x"`, `null`, `{"a":1} {"b":2}`, `{"a":`} {
		if _, err := ParseValues(raw); err == nil {
			t.Errorf("ParseValues(%q) succeeded, want an error", raw)
		}
	}
}

func TestValueReadsANestedSetting(t *testing.T) {
	values, _ := ParseValues(`{"agent":{"config":{"MODEL_ALIAS":"qwen2.5-7b"}},"replicas":3}`)
	if got, ok := Value(values, ModelAliasKey); !ok || got != "qwen2.5-7b" {
		t.Errorf("Value(MODEL_ALIAS) = %q, %v", got, ok)
	}
	if got, ok := Value(values, "replicas"); !ok || got != "3" {
		t.Errorf("Value(replicas) = %q, %v", got, ok)
	}
	if _, ok := Value(values, "agent.config.SCAN_INTERVAL"); ok {
		t.Error("Value() found a setting the document does not set")
	}
}

func TestLoadReportsAMissingChart(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "no-such-chart")); err == nil {
		t.Fatal("Load() of a missing chart succeeded")
	}
	empty := t.TempDir()
	if fields, err := Load(empty); err != nil || len(fields) != 0 {
		t.Fatalf("Load() of a chart without values.yaml = %v, %v; want no settings", fields, err)
	}
}

// TestRepositoryChartsDeclareValidConfigurations loads every chart in the
// monorepo checkout, so a broken `configurations` block fails here rather than
// silently hiding a chart's settings from the builder. Skipped outside the
// monorepo, where the chart repositories are not checked out alongside.
func TestRepositoryChartsDeclareValidConfigurations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..", "..")
	checked := 0
	for _, hub := range []string{"agent-charts", "app-charts"} {
		dirs, err := os.ReadDir(filepath.Join(root, hub, "charts"))
		if err != nil {
			continue
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			chart := filepath.Join(root, hub, "charts", dir.Name())
			fields, err := Load(chart)
			if err != nil {
				t.Errorf("%s/%s: %v", hub, dir.Name(), err)
			}
			t.Logf("%s/%s offers %d settings", hub, dir.Name(), len(fields))
			checked++
		}
	}
	if checked == 0 {
		t.Skip("chart repositories are not checked out next to AgentCert")
	}
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n") + "\n"
}
