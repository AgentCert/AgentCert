package model_config

import "errors"

// Gateway errors, returned by LiteLLMClient. All but ErrUnreachable mean the
// outcome of a write is known: LiteLLM answered and did not apply it.
var (
	// ErrRegistryUnavailable: LiteLLM runs without its database, so it cannot
	// store models. The models it serves from its config file keep working.
	ErrRegistryUnavailable = errors.New("the model registry needs LiteLLM's database: set LITELLM_DATABASE_URL and LITELLM_SALT_KEY (scripts/setup.sh does) and restart LiteLLM")
	// ErrUnauthorized: LiteLLM refused ACE's master key.
	ErrUnauthorized = errors.New("ACE can't authenticate to LiteLLM: check LITELLM_MASTER_KEY")
	// ErrNotFound: LiteLLM has no model with this ID.
	ErrNotFound = errors.New("LiteLLM has no model with this ID")
	// ErrRejected: LiteLLM refused the request; the message says why.
	ErrRejected = errors.New("LiteLLM rejected the request")
	// ErrUnreachable: no answer (network error, timeout, 5xx), so a write may
	// or may not have been applied. Reconcile finds out by the model's ID.
	ErrUnreachable = errors.New("LiteLLM did not answer")
)

// Service errors, shown to the user as they are.
var (
	ErrInvalidModel        = errors.New("invalid model")
	ErrAliasTaken          = errors.New("the alias is already in use; aliases are never reused, so pick another (e.g. add -v2)")
	ErrModelNotFound       = errors.New("no such model in this project")
	ErrModelInUse          = errors.New("the model is in use")
	ErrModelBusy           = errors.New("the model is still being registered or removed; try again in a minute")
	ErrModelUnavailable    = errors.New("model not available in this project")
	ErrPendingConfirmation = errors.New("the gateway didn't confirm the change; ACE finishes it automatically within a few minutes, so refresh the list to see the result")
)
