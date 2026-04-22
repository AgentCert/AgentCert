"""Patch install-agent/main.go to also set agent.config.AGENT_ID in the ConfigMap
during the helm upgrade --reuse-values step, so the sidecar volume mount contains
the real agent UUID and can inject it into LLM trace metadata.
"""
import sys

path = '/mnt/d/Studies/agent-charts/install-agent/main.go'

with open(path, 'r') as f:
    content = f.read()

# We target the CRLF variant first (Windows-edited file), then LF.
for nl in ('\r\n', '\n'):
    old = (
        '\t\t"--set", fmt.Sprintf("agentId=%s", agentID),' + nl +
        '\t\t"--timeout", config.Timeout,'
    )
    new = (
        '\t\t"--set", fmt.Sprintf("agentId=%s", agentID),' + nl +
        '\t\t// Also update the ConfigMap key so the sidecar can read AGENT_ID from the volume mount.' + nl +
        '\t\t"--set", fmt.Sprintf("agent.config.AGENT_ID=%s", agentID),' + nl +
        '\t\t"--timeout", config.Timeout,'
    )
    if old in content:
        content = content.replace(old, new, 1)
        with open(path, 'w') as f:
            f.write(content)
        print(f'PATCHED OK (nl={repr(nl)})')
        sys.exit(0)

print('NOT FOUND — showing context:')
idx = content.find('agentId=%s')
if idx >= 0:
    print(repr(content[max(0, idx - 100):idx + 200]))
else:
    print('agentId=%s not found at all')
sys.exit(1)
