package agent_registry

import (
	"testing"
	"time"
)

func TestAgentStatusIsValid(t *testing.T) {
	tests := []struct {
		status AgentStatus
		valid  bool
	}{
		{AgentStatusRegistered, true},
		{AgentStatusValidating, true},
		{AgentStatusActive, true},
		{AgentStatusInactive, true},
		{AgentStatusDeleted, true},
		{AgentStatus("INVALID"), false},
		{AgentStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("AgentStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestAgentStatusCanTransitionTo(t *testing.T) {
	tests := []struct {
		from  AgentStatus
		to    AgentStatus
		valid bool
	}{
		// From REGISTERED
		{AgentStatusRegistered, AgentStatusValidating, true},
		{AgentStatusRegistered, AgentStatusActive, true},
		{AgentStatusRegistered, AgentStatusDeleted, true},
		{AgentStatusRegistered, AgentStatusInactive, true},

		// From VALIDATING
		{AgentStatusValidating, AgentStatusActive, true},
		{AgentStatusValidating, AgentStatusInactive, true},
		{AgentStatusValidating, AgentStatusDeleted, true},
		{AgentStatusValidating, AgentStatusRegistered, false},

		// From ACTIVE
		{AgentStatusActive, AgentStatusInactive, true},
		{AgentStatusActive, AgentStatusDeleted, true},
		{AgentStatusActive, AgentStatusRegistered, false},
		{AgentStatusActive, AgentStatusValidating, true},

		// From INACTIVE
		{AgentStatusInactive, AgentStatusActive, true},
		{AgentStatusInactive, AgentStatusDeleted, true},
		{AgentStatusInactive, AgentStatusRegistered, false},

		// From DELETED (no transitions allowed)
		{AgentStatusDeleted, AgentStatusActive, false},
		{AgentStatusDeleted, AgentStatusRegistered, false},
	}

	for _, tt := range tests {
		name := string(tt.from) + " -> " + string(tt.to)
		t.Run(name, func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.valid {
				t.Errorf("CanTransitionTo(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.valid)
			}
		})
	}
}

func TestContainerImageFullImageName(t *testing.T) {
	tests := []struct {
		name     string
		image    ContainerImage
		expected string
	}{
		{
			name: "with registry and tag",
			image: ContainerImage{
				Registry:   "docker.io",
				Repository: "myorg/myagent",
				Tag:        "v1.0.0",
			},
			expected: "docker.io/myorg/myagent:v1.0.0",
		},
		{
			name: "with digest",
			image: ContainerImage{
				Registry:   "docker.io",
				Repository: "myorg/myagent",
				Digest:     "sha256:abc123",
			},
			expected: "docker.io/myorg/myagent@sha256:abc123",
		},
		{
			name: "without registry",
			image: ContainerImage{
				Repository: "myorg/myagent",
				Tag:        "latest",
			},
			expected: "myorg/myagent:latest",
		},
		{
			name: "with both tag and digest (digest preferred)",
			image: ContainerImage{
				Registry:   "gcr.io",
				Repository: "project/agent",
				Tag:        "v1.0.0",
				Digest:     "sha256:def456",
			},
			expected: "gcr.io/project/agent@sha256:def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.image.FullImageName(); got != tt.expected {
				t.Errorf("FullImageName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidatorValidateRegisterInput(t *testing.T) {
	validator := NewValidator(nil)

	tests := []struct {
		name    string
		input   RegisterAgentInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Name:         "my-agent",
				Version:      "1.0.0",
				Capabilities: []string{"llm-inference", "code-generation"},
			},
			wantErr: false,
		},
		{
			name: "missing project ID",
			input: RegisterAgentInput{
				Name:         "my-agent",
				Version:      "1.0.0",
				Capabilities: []string{"llm-inference"},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Version:      "1.0.0",
				Capabilities: []string{"llm-inference"},
			},
			wantErr: true,
		},
		{
			name: "invalid name format",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Name:         "123-invalid-start",
				Version:      "1.0.0",
				Capabilities: []string{"llm-inference"},
			},
			wantErr: true,
		},
		{
			name: "missing version",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Name:         "my-agent",
				Capabilities: []string{"llm-inference"},
			},
			wantErr: true,
		},
		{
			name: "invalid version format",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Name:         "my-agent",
				Version:      "invalid",
				Capabilities: []string{"llm-inference"},
			},
			wantErr: true,
		},
		{
			name: "empty capabilities",
			input: RegisterAgentInput{
				ProjectID: "project-123",
				Name:      "my-agent",
				Version:   "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "valid with container image",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Name:         "my-agent",
				Version:      "v1.0.0",
				Capabilities: []string{"llm-inference"},
				ContainerImage: &ContainerImageInput{
					Repository: "myorg/myagent",
					Tag:        "v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid container image (no tag or digest)",
			input: RegisterAgentInput{
				ProjectID:    "project-123",
				Name:         "my-agent",
				Version:      "v1.0.0",
				Capabilities: []string{"llm-inference"},
				ContainerImage: &ContainerImageInput{
					Repository: "myorg/myagent",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRegisterInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRegisterInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidSemVer(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0.0", true},
		{"0.1.0", true},
		{"v1.0.0", true},
		{"1.2.3-beta", true},
		{"1.2.3-alpha.1", true},
		{"1.2.3+build.123", true},
		{"1.2.3-beta+build", true},
		{"invalid", false},
		{"1.0", false},
		{"1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := isValidSemVer(tt.version); got != tt.valid {
				t.Errorf("isValidSemVer(%q) = %v, want %v", tt.version, got, tt.valid)
			}
		})
	}
}

func TestIsValidAgentName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"my-agent", true},
		{"myagent", true},
		{"my_agent", true},
		{"MyAgent", true},
		{"agent123", true},
		{"Agent-V2_test", true},
		{"123agent", false},
		{"-agent", false},
		{"_agent", false},
		{"agent@name", false},
		{"agent name", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidAgentName(tt.name); got != tt.valid {
				t.Errorf("isValidAgentName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestIsValidKubernetesName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"my-namespace", true},
		{"default", true},
		{"ns-123", true},
		{"a", true},
		{"MyNamespace", false}, // uppercase not allowed
		{"-ns", false},
		{"ns-", false},
		{"ns_name", false},    // underscore not allowed
		{"ns.name", false},    // dot not allowed
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidKubernetesName(tt.name); got != tt.valid {
				t.Errorf("isValidKubernetesName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestMultiValidationError(t *testing.T) {
	errs := &MultiValidationError{}

	if errs.HasErrors() {
		t.Error("New MultiValidationError should not have errors")
	}

	errs.Add("field1", "error1")
	errs.Add("field2", "error2")

	if !errs.HasErrors() {
		t.Error("MultiValidationError should have errors after Add")
	}

	if len(errs.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errs.Errors))
	}

	errStr := errs.Error()
	if errStr == "" {
		t.Error("Error() should return non-empty string")
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("Agent", "agent-123")

	if err.Resource != "Agent" {
		t.Errorf("Resource = %q, want %q", err.Resource, "Agent")
	}

	if err.ID != "agent-123" {
		t.Errorf("ID = %q, want %q", err.ID, "agent-123")
	}

	expectedMsg := "Agent with ID 'agent-123' not found"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestNewOperationError(t *testing.T) {
	originalErr := ErrAgentNotFound
	err := NewOperationError("GetAgent", "agent-123", originalErr)

	if err.Operation != "GetAgent" {
		t.Errorf("Operation = %q, want %q", err.Operation, "GetAgent")
	}

	if err.AgentID != "agent-123" {
		t.Errorf("AgentID = %q, want %q", err.AgentID, "agent-123")
	}

	if err.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}
}

func TestAgentModel(t *testing.T) {
	agent := &Agent{
		AgentID:      "test-agent-id",
		ProjectID:    "project-123",
		Name:         "Test Agent",
		Description:  "A test agent",
		Vendor:       "TestVendor",
		Version:      "1.0.0",
		Capabilities: []string{"llm-inference", "code-generation"},
		Status:       AgentStatusRegistered,
		ContainerImage: &ContainerImage{
			Registry:   "docker.io",
			Repository: "testvendor/testagent",
			Tag:        "v1.0.0",
		},
		Endpoint: &AgentEndpoint{
			DiscoveryType: EndpointDiscoveryManual,
			EndpointType:  EndpointTypeREST,
			URL:           "https://agent.example.com",
		},
		HealthCheck: &HealthCheckConfig{
			Enabled:     true,
			Path:        "/health",
			IntervalSec: 60,
			TimeoutSec:  10,
		},
		AuditInfo: AuditInfo{
			CreatedAt: time.Now(),
			CreatedBy: "user-123",
		},
	}

	// Verify fields
	if agent.AgentID != "test-agent-id" {
		t.Errorf("AgentID = %q, want %q", agent.AgentID, "test-agent-id")
	}

	if agent.ContainerImage.FullImageName() != "docker.io/testvendor/testagent:v1.0.0" {
		t.Errorf("FullImageName() = %q, want %q", 
			agent.ContainerImage.FullImageName(), 
			"docker.io/testvendor/testagent:v1.0.0")
	}

	if !agent.Status.IsValid() {
		t.Error("Agent status should be valid")
	}

	if !agent.Status.CanTransitionTo(AgentStatusActive) {
		t.Error("REGISTERED should be able to transition to ACTIVE")
	}
}

func TestCapabilityCategories(t *testing.T) {
	// Verify some expected categories exist
	categories := CapabilityCategories

	if _, ok := categories["detection"]; !ok {
		t.Error("Expected 'detection' category to exist")
	}

	if _, ok := categories["diagnosis"]; !ok {
		t.Error("Expected 'diagnosis' category to exist")
	}

	if _, ok := categories["remediation"]; !ok {
		t.Error("Expected 'remediation' category to exist")
	}

	// Verify detection capabilities
	detectionCaps := categories["detection"]
	found := false
	for _, cap := range detectionCaps {
		if cap == "pod-crash-detection" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'pod-crash-detection' capability in detection category")
	}
}
