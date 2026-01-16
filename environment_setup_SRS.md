# AgentCert / Litmus Environment Setup SRS

## 1. Purpose
This document captures the complete setup, configuration, and changes made to run the AgentCert fork of Litmus Chaos Center on Minikube (WSL2, Windows host). It serves as the single source of truth for reproducing the working environment and understanding the resulting architecture.

## 2. Target Environment
- Host: Windows with WSL2 (Ubuntu distro)
- Kubernetes: Minikube
- Tooling: `kubectl` (via `minikube kubectl` or WSL), `helm` optional, `docker`
- Registry: Uses public Litmus images (tag `ci` unless specified)

## 3. Namespaces and Roles
- Control plane namespace: `litmus-chaos`
- Managed/chaos namespace (agent/infrastructure): `litmus-exp`
- No additional NetworkPolicies beyond those embedded in manifests.

## 4. Core Services (control plane in `litmus-chaos`)
- Frontend: NodePort 9091 → container 8185
- GraphQL server: NodePort 9004 → container 8081 (gRPC on 8001)
- Auth server: NodePort 9005 → container 3001 (gRPC on 3031)
- MongoDB: external `host.minikube.internal:27017` (user `root`, pass `1234`, db `admin`)
- Nginx sidecar: proxies `/api` → 9004 and `/auth` → 9005; buffering disabled (`proxy_buffering off; proxy_request_buffering off; proxy_busy_buffers_size 1k;`)

## 5. Key Fixes Applied
- Corrected Auth gRPC port to `3031` everywhere (was `8001`).
- Disabled nginx buffering to avoid streaming timeouts.
- Updated portal server and auth to point to `host.minikube.internal` for Mongo.
- Port-forward script made kubeconfig-aware (autodetects cluster/namespace).
- Infra manifest (`litmus-exp`) subscriber now targets in-cluster GraphQL: `http://litmusportal-server-service.litmus-chaos.svc.cluster.local:9004/query` instead of localhost.
- Cleaned duplicate/old litmus deployments from `default` namespace and removed completed chaos jobs/pods from `litmus`.

## 6. Manifests and Scripts
- Control plane: `local-custom/k8s/litmus-installation.yaml`
- Infra/agent (namespace-scoped): `AgentCert/agent-cert-choas-litmus-chaos-enable.yml`
- Helper scripts: `local-custom/scripts/deploy.sh`, `local-custom/scripts/port-forward.sh`, `local-custom/scripts/cleanup.sh`

## 7. Configuration Highlights
- `litmusportal-server` envs:
  - `INFRA_DEPLOYMENTS`: `["app=chaos-exporter", "name=chaos-operator", "app=workflow-controller", "app=event-tracker"]`
  - `SUBSCRIBER_IMAGE`: `litmuschaos/litmusportal-subscriber:ci`
  - `EVENT_TRACKER_IMAGE`: `litmuschaos/litmusportal-event-tracker:ci`
  - Argo images pinned to `v3.3.1` (workflow-controller/argoexec)
- `litmusportal-auth-server` envs:
  - `LITMUS_GQL_GRPC_ENDPOINT=litmusportal-server-service`
  - `LITMUS_GQL_GRPC_PORT=8001`
  - `GRPC_PORT=3031`, REST `3001`, admin creds `admin / Sanjeev@123`
- Subscriber config (in `litmus-exp` ConfigMap):
  - `SERVER_ADDR`: `http://litmusportal-server-service.litmus-chaos.svc.cluster.local:9004/query`
  - `INFRA_SCOPE=namespace`, `SKIP_SSL_VERIFY=false`, `IS_INFRA_CONFIRMED=false`
- Subscriber secret (in `litmus-exp`):
  - `INFRA_ID=1be6b309-ce8c-4cae-88e2-d3c0b81430e0`
  - `ACCESS_KEY` from portal UI (example: `kBARzCiTyNYgH9bUb1T8Zfpj81y-imsa`)
- Workflow controller ConfigMap carries `instanceID: 1be6b309-ce8c-4cae-88e2-d3c0b81430e0`

## 8. Deployment Steps (clean install)
1. Start cluster: `minikube start --memory=8g --cpus=4` (adjust as needed).
2. Deploy control plane: `kubectl apply -f local-custom/k8s/litmus-installation.yaml` (namespace `litmus-chaos` is created by the manifest if absent).
3. (Optional) Port-forward for local UI/API access:
   - Frontend: `kubectl -n litmus-chaos port-forward svc/litmusportal-frontend-service 9091:9091`
   - Server: `kubectl -n litmus-chaos port-forward svc/litmusportal-server-service 9004:9004`
   - Auth: `kubectl -n litmus-chaos port-forward svc/litmusportal-auth-server-service 9005:9005`
   - Or run `local-custom/scripts/port-forward.sh` (auto-detects kube context/namespace).
4. Deploy infra/agent (namespace `litmus-exp`): `kubectl apply -f AgentCert/agent-cert-choas-litmus-chaos-enable.yml`.
5. Verify pods:
   - Control plane (`litmus-chaos`): frontend, server, auth, mongo, nginx sidecar.
   - Infra (`litmus-exp`): workflow-controller, chaos-operator, chaos-exporter, event-tracker, subscriber.
6. Confirm subscriber logs show WebSocket connect to the server and infra confirmation.
7. Open UI (via port-forward or NodePort) and ensure the infra with ID `1be6b309-ce8c-4cae-88e2-d3c0b81430e0` is ACTIVE (no longer PENDING).

## 9. Operational Notes
- Use `wsl -d Ubuntu -e kubectl ...` on Windows if the host shell lacks `kubectl`.
- Mongo must be reachable at `host.minikube.internal:27017`; credentials are baked into the manifest.
- If subscriber ever loops, verify `SERVER_ADDR` points to the cluster service and that the infra ID/access key match the UI registration.
- Nginx buffering is disabled to avoid websocket/stream issues between UI and backend.

## 10. Troubleshooting Quick Checks
- Pods status: `kubectl get pods -n litmus-chaos` and `kubectl get pods -n litmus-exp`
- Subscriber logs: `kubectl logs -n litmus-exp deploy/subscriber`
- Event tracker logs: `kubectl logs -n litmus-exp deploy/event-tracker`
- GraphQL health (inside cluster): `kubectl -n litmus-chaos port-forward svc/litmusportal-server-service 9004:9004` then curl `http://localhost:9004/query` for 400/405 (expected without body).
- Auth health: similar via port-forward on 9005.

## 11. Architecture Overview
- UI → Nginx → Auth (REST+gRPC) and GraphQL server (REST+gRPC)
- GraphQL server connects to MongoDB external service
- Infra namespace (`litmus-exp`) runs Argo workflow-controller, chaos-operator, exporter, event-tracker, subscriber
- Subscriber/event-tracker maintain websocket to GraphQL server using infra ID/access key; workflows execute via Argo + chaos-operator

## 12. Change Log (recent)
- Fixed gRPC port mismatch for auth (`3031`); aligned services and env vars.
- Disabled nginx buffering to prevent stream timeouts.
- Pointed Mongo to `host.minikube.internal` with explicit creds.
- Corrected infra subscriber `SERVER_ADDR` to in-cluster service (removes dependency on local port-forward).
- Cleaned stale default namespace deployments and completed chaos jobs in `litmus`.

## 13. Next Steps / TODOs
- Add CI guardrails (lint/validate manifests, kubeval/kubeconform).
- Parameterize secrets (infra access key, admin password) via external Secret management.
- Consider enabling TLS for server/auth and updating subscriber `SKIP_SSL_VERIFY` accordingly.
