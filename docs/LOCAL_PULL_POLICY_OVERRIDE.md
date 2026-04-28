# Local minikube: Override image pull policy to IfNotPresent

By default all AgentCert images use `imagePullPolicy: Always`.
On local minikube, images are pre-loaded via `minikube image load` and cannot
be pulled from Docker Hub at runtime (no pull happens with `IfNotPresent`).

To switch to `IfNotPresent` for a local minikube session, set these two values
in `local-custom/config/.env` before starting the GraphQL server:

```
INSTALL_AGENT_IMAGE_PULL_POLICY=IfNotPresent
INSTALL_APPLICATION_IMAGE_PULL_POLICY=IfNotPresent
```

Also run the following to update the running litmusportal-server pod:

```bash
kubectl set env deployment/litmusportal-server -n litmus-chaos \
  INSTALL_AGENT_IMAGE_PULL_POLICY=IfNotPresent \
  INSTALL_APPLICATION_IMAGE_PULL_POLICY=IfNotPresent
```

The sidecar and flash-agent pull policies are set at Helm install time.
To override those for local use, edit:

- `agent-charts/charts/flash-agent/values.yaml` line: `pullPolicy: Always` → `IfNotPresent`

Or pass at helm install time:
```bash
helm upgrade --set sidecar.image.pullPolicy=IfNotPresent \
             --set agent.containerImage.pullPolicy=IfNotPresent ...
```

Remember to revert to `Always` before pushing / building for Azure.
