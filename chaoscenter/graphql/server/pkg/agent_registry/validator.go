package agent_registry

import (
	"regexp"
	"strings"
)

// Validator validates agent registry inputs
type Validator struct {
	operator *Operator
}

// NewValidator creates a new Validator
func NewValidator(operator *Operator) *Validator {
	return &Validator{
		operator: operator,
	}
}

// ValidateRegisterInput validates input for agent registration
func (v *Validator) ValidateRegisterInput(input RegisterAgentInput) error {
	errs := &MultiValidationError{}

	// Project ID
	if strings.TrimSpace(input.ProjectID) == "" {
		errs.Add("projectId", "project ID is required")
	}

	// Name validation
	name := strings.TrimSpace(input.Name)
	if name == "" {
		errs.Add("name", "name is required")
	} else if len(name) < 3 || len(name) > 100 {
		errs.Add("name", "name must be between 3 and 100 characters")
	} else if !isValidAgentName(name) {
		errs.Add("name", "name must start with a letter and contain only alphanumeric characters, hyphens, and underscores")
	}

	// Version validation
	if strings.TrimSpace(input.Version) == "" {
		errs.Add("version", "version is required")
	} else if !isValidSemVer(input.Version) {
		errs.Add("version", "version must follow semantic versioning (e.g., 1.0.0, 1.2.3-beta)")
	}

	// Capabilities validation
	if len(input.Capabilities) == 0 {
		errs.Add("capabilities", "at least one capability is required")
	} else {
		for i, cap := range input.Capabilities {
			if strings.TrimSpace(cap) == "" {
				errs.Add("capabilities", "capability at index "+string(rune('0'+i))+" is empty")
			}
		}
	}

	// Container image validation (if provided)
	if input.ContainerImage != nil {
		v.validateContainerImage(input.ContainerImage, errs)
	}

	// Endpoint validation (if provided)
	if input.Endpoint != nil {
		v.validateEndpoint(input.Endpoint, errs)
	}

	// Health check validation (if provided)
	if input.HealthCheck != nil {
		v.validateHealthCheck(input.HealthCheck, errs)
	}

	// Namespace validation
	if input.Namespace != "" && !isValidKubernetesName(input.Namespace) {
		errs.Add("namespace", "namespace must be a valid Kubernetes name")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateUpdateInput validates input for agent update
func (v *Validator) ValidateUpdateInput(input UpdateAgentInput) error {
	errs := &MultiValidationError{}

	// Version validation (if provided)
	if input.Version != nil && *input.Version != "" {
		if !isValidSemVer(*input.Version) {
			errs.Add("version", "version must follow semantic versioning (e.g., 1.0.0, 1.2.3-beta)")
		}
	}

	// Capabilities validation (if provided)
	if input.Capabilities != nil && len(input.Capabilities) > 0 {
		for i, cap := range input.Capabilities {
			if strings.TrimSpace(cap) == "" {
				errs.Add("capabilities", "capability at index "+string(rune('0'+i))+" is empty")
			}
		}
	}

	// Container image validation (if provided)
	if input.ContainerImage != nil {
		v.validateContainerImage(input.ContainerImage, errs)
	}

	// Endpoint validation (if provided)
	if input.Endpoint != nil {
		v.validateEndpoint(input.Endpoint, errs)
	}

	// Health check validation (if provided)
	if input.HealthCheck != nil {
		v.validateHealthCheck(input.HealthCheck, errs)
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// validateContainerImage validates container image input
func (v *Validator) validateContainerImage(img *ContainerImageInput, errs *MultiValidationError) {
	if strings.TrimSpace(img.Repository) == "" {
		errs.Add("containerImage.repository", "repository is required")
	}
	if strings.TrimSpace(img.Tag) == "" && strings.TrimSpace(img.Digest) == "" {
		errs.Add("containerImage.tag", "either tag or digest is required")
	}
	if img.PullPolicy != "" && !isValidPullPolicy(img.PullPolicy) {
		errs.Add("containerImage.pullPolicy", "pullPolicy must be Always, IfNotPresent, or Never")
	}
}

// validateEndpoint validates endpoint input
func (v *Validator) validateEndpoint(ep *AgentEndpointInput, errs *MultiValidationError) {
	discoveryType := EndpointDiscoveryType(ep.DiscoveryType)
	if discoveryType != EndpointDiscoveryAuto && discoveryType != EndpointDiscoveryManual {
		errs.Add("endpoint.discoveryType", "discoveryType must be AUTO or MANUAL")
	}

	endpointType := EndpointType(ep.EndpointType)
	if endpointType != EndpointTypeREST && endpointType != EndpointTypeGRPC {
		errs.Add("endpoint.endpointType", "endpointType must be REST or GRPC")
	}

	if discoveryType == EndpointDiscoveryManual {
		if strings.TrimSpace(ep.URL) == "" {
			errs.Add("endpoint.url", "URL is required for MANUAL discovery")
		} else if !isValidURL(ep.URL) {
			errs.Add("endpoint.url", "URL must be a valid HTTP(S) or gRPC URL")
		}
	}

	if discoveryType == EndpointDiscoveryAuto {
		if strings.TrimSpace(ep.ServiceName) == "" {
			errs.Add("endpoint.serviceName", "serviceName is required for AUTO discovery")
		}
		if ep.Port <= 0 || ep.Port > 65535 {
			errs.Add("endpoint.port", "port must be between 1 and 65535")
		}
	}
}

// validateHealthCheck validates health check configuration
func (v *Validator) validateHealthCheck(hc *HealthCheckConfigInput, errs *MultiValidationError) {
	if !hc.Enabled {
		return
	}

	if hc.IntervalSec < 10 {
		errs.Add("healthCheck.intervalSec", "intervalSec must be at least 10 seconds")
	}
	if hc.TimeoutSec < 1 {
		errs.Add("healthCheck.timeoutSec", "timeoutSec must be at least 1 second")
	}
	if hc.TimeoutSec >= hc.IntervalSec {
		errs.Add("healthCheck.timeoutSec", "timeoutSec must be less than intervalSec")
	}
	if hc.MaxRetries < 1 {
		errs.Add("healthCheck.maxRetries", "maxRetries must be at least 1")
	}
}

// Helper functions

var agentNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

func isValidAgentName(name string) bool {
	return agentNameRegex.MatchString(name)
}

var semVerRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)

func isValidSemVer(version string) bool {
	return semVerRegex.MatchString(version)
}

var k8sNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func isValidKubernetesName(name string) bool {
	return len(name) <= 63 && k8sNameRegex.MatchString(name)
}

func isValidPullPolicy(policy string) bool {
	switch policy {
	case "Always", "IfNotPresent", "Never":
		return true
	}
	return false
}

var urlRegex = regexp.MustCompile(`^(https?|grpc)://[^\s/$.?#].[^\s]*$`)

func isValidURL(url string) bool {
	return urlRegex.MatchString(url)
}
