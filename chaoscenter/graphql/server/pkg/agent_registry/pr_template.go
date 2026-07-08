package agent_registry

import (
	"fmt"
	"strings"
)

// GenerateCommunityPRTemplate returns a Markdown PR description for a community catalog contribution.
func GenerateCommunityPRTemplate(agent *Agent) string {
	ownerName := ""
	ownerEmail := ""
	if agent.AgentOwner != nil {
		ownerName = agent.AgentOwner.Name
		ownerEmail = agent.AgentOwner.Email
	}

	caps := strings.Join(agent.Capabilities, ", ")
	if caps == "" {
		caps = "(none declared)"
	}

	description := ""
	if agent.SpecDescription != nil && agent.SpecDescription.Short != "" {
		description = agent.SpecDescription.Short
	}

	displayName := agent.DisplayName
	if displayName == "" {
		displayName = agent.Name
	}

	return fmt.Sprintf(`## New Community Agent: %s

### Summary
- **Agent Name:** %s
- **Version:** %s
- **Owner:** %s <%s>

### Agent Description
%s

### Capabilities
%s

### Review Checklist
- [ ] agent.yaml is valid
- [ ] All capability keys exist in catalog/capabilities/
- [ ] No hardcoded secrets in values.yaml
- [ ] Image is publicly pullable
- [ ] Owner has been notified of review requirements
`,
		displayName,
		agent.Name,
		agent.Version,
		ownerName,
		ownerEmail,
		description,
		caps,
	)
}
