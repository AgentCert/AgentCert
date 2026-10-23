// Package faultconfig resolves a fault's declared settings from trusted hub
// files. Defaults remain in fault.yaml; the CSV supplies presentation and rules.
package faultconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/chartconfig"
	yamlv3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

const SourceAnnotation = "agentcert.io/fault-source"

var ErrNotFound = errors.New("fault definition not found")
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var folderName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type Source struct {
	HubID    string `json:"hubID"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

type Definition struct {
	Source    Source
	CSV       string
	Fault     string
	Fields    []chartconfig.Field
	Defaults  map[string]corev1.EnvVar
	HasSchema bool
	Revision  string
}

// Cache is bounded and keyed by content, not by a TTL. A mounted file or hub
// sync takes effect on the next read, including edits that preserve timestamps.
type Cache struct {
	parsed *lru.Cache[string, *Definition]
}

func NewCache() *Cache {
	parsed, _ := lru.New[string, *Definition](256)
	return &Cache{parsed: parsed}
}

// Directory rejects traversal and symlinks escaping the selected hub.
func Directory(root, category, name string) (string, error) {
	if !folderName.MatchString(category) || !folderName.MatchString(name) {
		return "", errors.New("fault category and name must be single directory names")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, category, name))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, dir)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("fault directory escapes the selected hub")
	}
	return dir, nil
}

func (c *Cache) Load(dir, name string) (*Definition, error) {
	csv, err := ReadFile(dir, name+".chartserviceversion.yaml")
	if err != nil {
		return nil, fmt.Errorf("read fault metadata: %w", err)
	}
	fault, err := ReadFile(dir, "fault.yaml")
	if err != nil {
		return nil, fmt.Errorf("read fault definition: %w", err)
	}
	digest := sha256.New()
	digest.Write(csv)
	digest.Write([]byte{0})
	digest.Write(fault)
	revision := hex.EncodeToString(digest.Sum(nil))
	if cached, ok := c.parsed.Get(revision); ok {
		return cached, nil
	}
	definition, err := Parse(csv, fault)
	if err != nil {
		return nil, err
	}
	definition.Revision = revision
	c.parsed.Add(revision, definition)
	return definition, nil
}

// ReadFile also checks file symlinks; validating the directory alone would
// still let an untrusted hub point engine.yaml outside its bundle.
func ReadFile(dir, name string) ([]byte, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	file, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("fault file escapes its bundle")
	}
	return os.ReadFile(file)
}

func Parse(csv, fault []byte) (*Definition, error) {
	var metadata struct {
		Spec map[string]interface{} `yaml:"spec"`
	}
	if err := yamlv3.Unmarshal(csv, &metadata); err != nil {
		return nil, fmt.Errorf("invalid fault metadata: %w", err)
	}
	var experiment struct {
		Kind string `json:"kind"`
		Spec struct {
			Definition struct {
				Env []corev1.EnvVar `json:"env"`
			} `json:"definition"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(fault, &experiment); err != nil {
		return nil, fmt.Errorf("invalid fault.yaml: %w", err)
	}
	if experiment.Kind != "ChaosExperiment" {
		return nil, errors.New("fault.yaml must contain a ChaosExperiment")
	}
	definition := &Definition{CSV: string(csv), Fault: string(fault), Defaults: make(map[string]corev1.EnvVar)}
	values := make(map[string]interface{})
	for _, env := range experiment.Spec.Definition.Env {
		if _, exists := definition.Defaults[env.Name]; exists {
			return nil, fmt.Errorf("fault.yaml repeats environment variable %s", env.Name)
		}
		definition.Defaults[env.Name] = env
		if env.ValueFrom == nil {
			values[env.Name] = env.Value
		}
	}
	block, present := metadata.Spec["configurations"]
	definition.HasSchema = present
	if !present {
		return definition, nil // Legacy hubs retain their existing tuning flow.
	}
	if block == nil {
		return nil, errors.New("spec.configurations must be a list, use [] to offer no settings")
	}
	values["configurations"] = block
	encoded, err := yamlv3.Marshal(values)
	if err != nil {
		return nil, err
	}
	definition.Fields, err = chartconfig.Parse(encoded)
	if err != nil {
		return nil, fmt.Errorf("fault configurations: %w", err)
	}
	for _, field := range definition.Fields {
		if !envName.MatchString(field.Key) || field.Type == chartconfig.TypeModel || platformEnv(field.Key) {
			return nil, fmt.Errorf("%s is not a user-configurable fault environment variable", field.Key)
		}
	}
	return definition, nil
}

func platformEnv(name string) bool {
	switch name {
	case "EXPERIMENT_NAME", "CHAOSENGINE", "CHAOS_NAMESPACE", "CHAOS_UID", "INSTANCE_ID", "POD_NAME", "CHAOS_SERVICE_ACCOUNT", "LIB_IMAGE", "LIB_IMAGE_PULL_POLICY", "TARGETS":
		return true
	}
	return false
}

// ValidateEnvironment checks the effective values, including omitted defaults.
// Hidden variables and references can be retained, but not edited via the form.
func ValidateEnvironment(definition *Definition, envs []corev1.EnvVar) error {
	if !definition.HasSchema {
		return nil
	}
	fields := make(map[string]bool, len(definition.Fields))
	values := make(map[string]interface{}, len(definition.Fields))
	for _, field := range definition.Fields {
		fields[field.Key] = true
		values[field.Key] = field.Default
	}
	seen := make(map[string]bool, len(envs))
	for _, env := range envs {
		if seen[env.Name] {
			return fmt.Errorf("environment variable %s is repeated", env.Name)
		}
		seen[env.Name] = true
		if fields[env.Name] {
			if env.ValueFrom != nil {
				return fmt.Errorf("%s must have a literal value for validation", env.Name)
			}
			values[env.Name] = env.Value
			continue
		}
		// The existing builder adds LIB_IMAGE after applying registry overrides.
		if env.Name == "LIB_IMAGE" {
			continue
		}
		original, exists := definition.Defaults[env.Name]
		if !exists || !reflect.DeepEqual(original, env) {
			return fmt.Errorf("%s is not a setting this fault offers", env.Name)
		}
	}
	return chartconfig.Validate(definition.Fields, values)
}
