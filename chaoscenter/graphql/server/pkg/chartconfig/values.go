package chartconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ParseValues decodes the settings document the builder attaches to an
// install step: a JSON object shaped like a Helm values file, e.g.
// {"agent":{"config":{"SCAN_INTERVAL":"90"}}}.
func ParseValues(raw string) (map[string]interface{}, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	// Keeps 90 and 90.5 distinguishable, so an integer setting rejects 90.5.
	decoder.UseNumber()
	var values map[string]interface{}
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("settings are not a JSON object: %w", err)
	}
	if values == nil {
		return nil, errors.New("settings are not a JSON object")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("settings contain data after the JSON object")
	}
	return values, nil
}

// Validate checks every value in a settings document against the chart's
// declared fields. Settings that are left out keep the chart default, so only
// the values present are checked.
func Validate(fields []Field, values map[string]interface{}) error {
	byKey := make(map[string]Field, len(fields))
	for _, field := range fields {
		byKey[field.Key] = field
	}

	leaves := make(map[string]interface{})
	if err := flatten("", values, leaves); err != nil {
		return err
	}
	keys := make([]string, 0, len(leaves))
	for key := range leaves {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var errs []error
	for _, key := range keys {
		field, ok := byKey[key]
		if !ok {
			errs = append(errs, fmt.Errorf("%q is not a setting this chart offers", key))
			continue
		}
		if err := field.checkValue(leaves[key]); err != nil {
			errs = append(errs, fmt.Errorf("%s (%s) %w", field.Label, field.Key, err))
		}
	}
	return errors.Join(errs...)
}

// Value returns the text of the setting at key, if the document sets it.
func Value(values map[string]interface{}, key string) (string, bool) {
	raw, ok := lookup(values, key)
	if !ok {
		return "", false
	}
	text, ok := textOf(raw)
	return text, ok
}

func flatten(prefix string, node map[string]interface{}, leaves map[string]interface{}) error {
	for name, value := range node {
		if name == "" || strings.Contains(name, ".") {
			return fmt.Errorf("%q must be a single path segment; use nested objects for dotted paths", name)
		}
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}
		switch child := value.(type) {
		case map[string]interface{}:
			if len(child) == 0 {
				return fmt.Errorf("%q must hold settings, not an empty object", key)
			}
			if err := flatten(key, child, leaves); err != nil {
				return err
			}
		case []interface{}:
			return fmt.Errorf("%q is a list; only single values can be set", key)
		case nil:
			return fmt.Errorf("%q has no value", key)
		default:
			leaves[key] = child
		}
	}
	return nil
}

// checkValue validates one value, either a decoded JSON value from the
// builder or the chart's own default.
func (f Field) checkValue(v interface{}) error {
	if err := f.checkKind(v); err != nil {
		return err
	}
	text, _ := textOf(v)

	if f.Kind == KindString && strings.Contains(text, "{{") {
		// Argo would expand it as a workflow template expression.
		return errors.New("must not contain \"{{\"")
	}
	if f.Required && strings.TrimSpace(text) == "" {
		return errors.New("is required")
	}

	switch f.Type {
	case TypeInteger:
		n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return fmt.Errorf("must be a whole number, got %q", text)
		}
		return f.checkRange(float64(n), text)
	case TypeNumber:
		n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("must be a number, got %q", text)
		}
		return f.checkRange(n, text)
	case TypeBoolean:
		if text != "true" && text != "false" {
			return fmt.Errorf("must be true or false, got %q", text)
		}
	case TypeSelect:
		for _, option := range f.Options {
			if text == option {
				return nil
			}
		}
		return fmt.Errorf("must be one of %s, got %q", strings.Join(f.Options, ", "), text)
	case TypeModel:
		if strings.TrimSpace(text) == "" {
			return errors.New("must name a model; leave the setting out to use the platform default")
		}
	case TypeString, TypeText:
		if f.Pattern != "" && !regexp.MustCompile(f.Pattern).MatchString(text) {
			return fmt.Errorf("must match %s, got %q", f.Pattern, text)
		}
	}
	return nil
}

// checkKind requires the value's JSON type to be the chart default's type.
func (f Field) checkKind(v interface{}) error {
	switch f.Kind {
	case KindString:
		if _, ok := v.(string); !ok {
			return errors.New("must be written as text (a JSON string)")
		}
	case KindBoolean:
		if _, ok := v.(bool); !ok {
			return errors.New("must be true or false (a JSON boolean)")
		}
	case KindInteger, KindNumber:
		if !isNumber(v) {
			return errors.New("must be written as a number (a JSON number)")
		}
		if f.Kind == KindInteger {
			text, _ := textOf(v)
			if _, err := strconv.ParseInt(text, 10, 64); err != nil {
				return errors.New("must be a whole number")
			}
		}
	}
	return nil
}

func (f Field) checkRange(n float64, text string) error {
	if f.Min != nil && n < *f.Min {
		return fmt.Errorf("must be at least %s, got %s", formatScalar(*f.Min), text)
	}
	if f.Max != nil && n > *f.Max {
		return fmt.Errorf("must be at most %s, got %s", formatScalar(*f.Max), text)
	}
	return nil
}

func isNumber(v interface{}) bool {
	switch v.(type) {
	case json.Number, int64, float64:
		return true
	}
	return false
}

func textOf(v interface{}) (string, bool) {
	switch value := v.(type) {
	case json.Number:
		return value.String(), true
	case string, bool, int64, float64:
		return formatScalar(value), true
	}
	return "", false
}
