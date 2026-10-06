// Package chartconfig reads the settings an agent or application chart offers
// to the experiment builder, and validates the values the builder sends back.
//
// A chart declares its settings in a top-level `configurations` block of its
// own values.yaml. Each entry names a value that already exists in that file;
// the value found there is the setting's default, and its YAML type is the type
// the user's value is written back as. Defaults are therefore never duplicated,
// and a setting stored as "60" (an env var) stays a string while `replicas: 1`
// stays a number:
//
//	agent:
//	  config:
//	    SCAN_INTERVAL: "60"
//	configurations:
//	  - key: agent.config.SCAN_INTERVAL
//	    label: Scan interval (seconds)
//	    type: integer
//	    min: 1
//
// The builder sends the chosen values to the install step as one JSON document
// (see ParseValues), which the installer passes to Helm as a values file. The
// server validates that document against the declarations on every save and
// run, so a chart is never installed with a value it did not offer.
package chartconfig

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FieldType is how the builder renders a setting.
type FieldType string

const (
	TypeString  FieldType = "string"
	TypeText    FieldType = "text"
	TypeInteger FieldType = "integer"
	TypeNumber  FieldType = "number"
	TypeBoolean FieldType = "boolean"
	TypeSelect  FieldType = "select"
	// TypeModel is an LLM model alias picked from the platform's model list.
	TypeModel FieldType = "model"
)

// ValueKind is the JSON type a setting's value is written as. It is the YAML
// type of the chart's own default.
type ValueKind string

const (
	KindString  ValueKind = "string"
	KindInteger ValueKind = "integer"
	KindNumber  ValueKind = "number"
	KindBoolean ValueKind = "boolean"
)

// Field is one setting a chart offers.
type Field struct {
	// Key is the dotted Helm values path, e.g. agent.config.SCAN_INTERVAL.
	Key         string
	Label       string
	Description string
	Type        FieldType
	Kind        ValueKind
	// Default is the chart's value at Key: a string, int64, float64 or bool.
	Default  interface{}
	Required bool
	Min      *float64
	Max      *float64
	Pattern  string
	Options  []string
	Group    string
	Advanced bool
}

// declaration is one `configurations` entry as written in values.yaml.
type declaration struct {
	Key         string   `yaml:"key"`
	Label       string   `yaml:"label"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Min         *float64 `yaml:"min"`
	Max         *float64 `yaml:"max"`
	Pattern     string   `yaml:"pattern"`
	Options     []string `yaml:"options"`
	Group       string   `yaml:"group"`
	Advanced    bool     `yaml:"advanced"`
}

// ErrChartNotFound is returned by Load for a chart directory that does not exist.
var ErrChartNotFound = errors.New("chart not found")

// Load reads the settings declared by the chart in chartDir. A chart without a
// values.yaml or without a `configurations` block offers no settings.
func Load(chartDir string) ([]Field, error) {
	if _, err := os.Stat(chartDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrChartNotFound, chartDir)
		}
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse reads the settings declared in the contents of a values.yaml.
func Parse(valuesYAML []byte) ([]Field, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(valuesYAML, &doc); err != nil {
		return nil, fmt.Errorf("values.yaml is not valid YAML: %w", err)
	}
	block, ok := doc["configurations"]
	if !ok || block == nil {
		return nil, nil
	}

	// Decoded strictly, so a misspelt attribute (e.g. `requried: true`) is an
	// error rather than a silently ignored line.
	raw, err := yaml.Marshal(block)
	if err != nil {
		return nil, err
	}
	var decls []declaration
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&decls); err != nil {
		return nil, fmt.Errorf("configurations: %w", err)
	}

	fields := make([]Field, 0, len(decls))
	seen := make(map[string]bool, len(decls))
	var errs []error
	for i, decl := range decls {
		field, err := newField(decl, doc)
		if err != nil {
			errs = append(errs, fmt.Errorf("configurations[%d] (%s): %w", i, decl.Key, err))
			continue
		}
		if seen[field.Key] {
			errs = append(errs, fmt.Errorf("configurations[%d] (%s): declared more than once", i, decl.Key))
			continue
		}
		seen[field.Key] = true
		fields = append(fields, field)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return fields, nil
}

func newField(decl declaration, doc map[string]interface{}) (Field, error) {
	key := strings.TrimSpace(decl.Key)
	if !isValidKey(key) {
		return Field{}, errors.New("key must be a dotted values path such as agent.config.SCAN_INTERVAL")
	}
	if IsPlatformOwned(key) && !IsUserSettable(key) {
		return Field{}, errors.New("this value is set by the platform on every install and cannot be offered to users")
	}
	if IsSecretKey(key) {
		return Field{}, errors.New("secrets cannot be offered as settings: their values would be stored in the experiment in plain text")
	}

	raw, found := lookup(doc, key)
	if !found {
		return Field{}, errors.New("key does not exist in values.yaml; add it there with its default value first")
	}
	def, kind, ok := scalar(raw)
	if !ok {
		return Field{}, errors.New("key must hold a single value (string, number or boolean), not a list or map")
	}

	field := Field{
		Key:         key,
		Label:       strings.TrimSpace(decl.Label),
		Description: strings.TrimSpace(decl.Description),
		Type:        FieldType(strings.TrimSpace(decl.Type)),
		Kind:        kind,
		Default:     def,
		Required:    decl.Required,
		Min:         decl.Min,
		Max:         decl.Max,
		Pattern:     decl.Pattern,
		Options:     decl.Options,
		Group:       strings.TrimSpace(decl.Group),
		Advanced:    decl.Advanced,
	}
	if field.Label == "" {
		field.Label = key[strings.LastIndex(key, ".")+1:]
	}
	if err := field.checkDeclaration(); err != nil {
		return Field{}, err
	}
	return field, nil
}

// checkDeclaration rejects a declaration whose attributes contradict each
// other or the chart's default, so a chart author finds out at review time
// rather than when a user's valid-looking value is rejected.
func (f Field) checkDeclaration() error {
	switch f.Type {
	case TypeString, TypeText, TypeModel:
		if f.Kind != KindString {
			return fmt.Errorf("type %q needs a string default, got %s", f.Type, f.Kind)
		}
	case TypeSelect:
		if f.Kind != KindString {
			return fmt.Errorf("type %q needs a string default, got %s", f.Type, f.Kind)
		}
		if len(f.Options) == 0 {
			return errors.New("type select needs options")
		}
	case TypeInteger, TypeNumber, TypeBoolean:
	case "":
		return errors.New("type is required")
	default:
		return fmt.Errorf("unknown type %q (use string, text, integer, number, boolean, select or model)", f.Type)
	}
	if len(f.Options) > 0 && f.Type != TypeSelect {
		return errors.New("options are only allowed for type select")
	}
	if (f.Min != nil || f.Max != nil) && f.Type != TypeInteger && f.Type != TypeNumber {
		return errors.New("min and max are only allowed for type integer or number")
	}
	if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
		return errors.New("min is greater than max")
	}
	for _, bound := range []*float64{f.Min, f.Max} {
		if bound != nil && (math.IsNaN(*bound) || math.IsInf(*bound, 0)) {
			return errors.New("min and max must be finite numbers")
		}
	}
	if f.Pattern != "" {
		if f.Type != TypeString && f.Type != TypeText {
			return errors.New("pattern is only allowed for type string or text")
		}
		if _, err := regexp.Compile(f.Pattern); err != nil {
			return fmt.Errorf("pattern does not compile: %w", err)
		}
	}
	// A model setting defaults to the platform's model, not the chart's
	// placeholder, so its default is not held to the field's own rules.
	if f.Type == TypeModel {
		return nil
	}
	if err := f.checkValue(f.Default); err != nil {
		return fmt.Errorf("the default in values.yaml is not a valid value: %w", err)
	}
	return nil
}

// FormatDefault returns the field's default as text.
func (f Field) FormatDefault() string {
	return formatScalar(f.Default)
}

func isValidKey(key string) bool {
	if key == "" {
		return false
	}
	for _, segment := range strings.Split(key, ".") {
		if segment == "" {
			return false
		}
	}
	return true
}

// lookup walks a dotted path through decoded YAML or JSON maps.
func lookup(doc map[string]interface{}, key string) (interface{}, bool) {
	var current interface{} = doc
	for _, segment := range strings.Split(key, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// scalar normalises a decoded YAML scalar and reports its kind.
func scalar(v interface{}) (interface{}, ValueKind, bool) {
	switch value := v.(type) {
	case string:
		return value, KindString, true
	case bool:
		return value, KindBoolean, true
	case int:
		return int64(value), KindInteger, true
	case int64:
		return value, KindInteger, true
	case uint64:
		if value > math.MaxInt64 {
			return nil, "", false
		}
		return int64(value), KindInteger, true
	case float64:
		return value, KindNumber, true
	default:
		return nil, "", false
	}
}

func formatScalar(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return fmt.Sprint(value)
	}
}
