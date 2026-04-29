import sys

path = '/mnt/d/Studies/AgentCert/chaoscenter/graphql/server/pkg/observability/langfuse_tracer.go'

with open(path, 'r') as f:
    content = f.read()

orig_len = len(content)

# Fix 1: tab-merged Input map agentVersion line
# Replace the tab between agentPlatform, agentVersion, serviceAccount lines with newlines
old1 = '"agentPlatform":  details.AgentPlatform,\t\t\t\t"agentVersion":   details.AgentVersion,\t\t\t"serviceAccount": details.AgentServiceAccount,'
new1 = '"agentPlatform":  details.AgentPlatform,\n\t\t\t\t"agentVersion":   details.AgentVersion,\n\t\t\t\t"serviceAccount": details.AgentServiceAccount,'
c1 = content.replace(old1, new1)
if c1 == content:
    print("FIX1 NOT FOUND - trying alternate tab counts")
    # try different tab counts
    for ti in range(1, 8):
        for tj in range(1, 8):
            old1x = '"agentPlatform":  details.AgentPlatform,' + '\t'*ti + '"agentVersion":   details.AgentVersion,' + '\t'*tj + '"serviceAccount": details.AgentServiceAccount,'
            c1x = content.replace(old1x, new1)
            if c1x != content:
                print(f"FIX1 found with ti={ti} tj={tj}")
                c1 = c1x
                break
        if c1 != content:
            break
else:
    print("FIX1 OK")
content = c1

# Fix 2: tab-merged Metadata map agent_version line
old2 = '"agent_platform":      details.AgentPlatform,\t\t\t\t\t"agent_version":       details.AgentVersion,\t\t\t"agent_id":            details.AgentID,'
new2 = '"agent_platform":      details.AgentPlatform,\n\t\t\t\t"agent_version":       details.AgentVersion,\n\t\t\t\t"agent_id":            details.AgentID,'
c2 = content.replace(old2, new2)
if c2 == content:
    print("FIX2 NOT FOUND - trying alternate tab counts")
    for ti in range(1, 8):
        for tj in range(1, 8):
            old2x = '"agent_platform":      details.AgentPlatform,' + '\t'*ti + '"agent_version":       details.AgentVersion,' + '\t'*tj + '"agent_id":            details.AgentID,'
            c2x = content.replace(old2x, new2)
            if c2x != content:
                print(f"FIX2 found with ti={ti} tj={tj}")
                c2 = c2x
                break
        if c2 != content:
            break
else:
    print("FIX2 OK")
content = c2

# Fix 3: add agent_version to experiment_context SPAN metadata
old3 = '"agent_platform":  expCtx.AgentPlatform,\n\t\t\t\t"experiment_id":   expCtx.ExperimentID,'
new3 = '"agent_platform":  expCtx.AgentPlatform,\n\t\t\t\t"agent_version":   expCtx.AgentVersion,\n\t\t\t\t"experiment_id":   expCtx.ExperimentID,'
c3 = content.replace(old3, new3)
if c3 == content:
    print("FIX3 NOT FOUND")
else:
    print("FIX3 OK")
content = c3

# Fix 4: add attributes sub-dict to fault SPAN metaData
old4 = '\t\t\t"tokens_consumed": 0,\n\t\t}\n\n\t\tpayload := &agent_registry.LangfuseObservationPayload{'
new4 = '\t\t\t"tokens_consumed": 0,\n\t\t\t"attributes": map[string]interface{}{\n\t\t\t\t"fault.target_namespace": expCtx.Namespace,\n\t\t\t\t"fault.target_label":     fname,\n\t\t\t},\n\t\t}\n\n\t\tpayload := &agent_registry.LangfuseObservationPayload{'
c4 = content.replace(old4, new4)
if c4 == content:
    print("FIX4 NOT FOUND")
else:
    print("FIX4 OK")
content = c4

with open(path, 'w') as f:
    f.write(content)

print(f"Done. Original length: {orig_len}, new length: {len(content)}")
