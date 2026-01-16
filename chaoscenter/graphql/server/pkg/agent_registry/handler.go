package agent_registry

// Handler provides GraphQL resolver handlers for Agent Registry operations.
// It transforms GraphQL requests to service layer calls and handles response formatting.
type Handler struct {
	service Service
}

// NewHandler creates a new Handler instance with the provided Service.
func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}
