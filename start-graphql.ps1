$env:VERSION = "3.0.0"
$env:INFRA_DEPLOYMENTS = "false"
$env:DB_SERVER = "mongodb://localhost:27017"
$env:JWT_SECRET = "litmus-portal@123"
$env:DB_USER = "admin"
$env:DB_PASSWORD = "1234"
$env:SELF_AGENT = "false"
$env:INFRA_COMPATIBLE_VERSIONS = '["3.0.0"]'
$env:ALLOWED_ORIGINS = '^(http://|https://|)(localhost|host\.docker\.internal)(:[0-9]+|)'
$env:SKIP_SSL_VERIFY = "true"
$env:ENABLE_GQL_INTROSPECTION = "true"
$env:INFRA_SCOPE = "cluster"
$env:LITMUS_AUTH_GRPC_ENDPOINT = "localhost"
$env:LITMUS_AUTH_GRPC_PORT = "3030"
$env:ENABLE_INTERNAL_TLS = "false"
$env:DEFAULT_HUB_GIT_URL = "https://github.com/sharmadeep2/chaos-charts"
$env:DEFAULT_HUB_BRANCH_NAME = "master"
$env:SUBSCRIBER_IMAGE = "litmuschaos/litmusportal-subscriber:3.0.0"
$env:EVENT_TRACKER_IMAGE = "litmuschaos/litmusportal-event-tracker:3.0.0"
$env:ARGO_WORKFLOW_CONTROLLER_IMAGE = "litmuschaos/workflow-controller:v3.3.1"
$env:ARGO_WORKFLOW_EXECUTOR_IMAGE = "litmuschaos/argoexec:v3.3.1"
$env:LITMUS_CHAOS_OPERATOR_IMAGE = "litmuschaos/chaos-operator:3.0.0"
$env:LITMUS_CHAOS_RUNNER_IMAGE = "litmuschaos/chaos-runner:3.0.0"
$env:LITMUS_CHAOS_EXPORTER_IMAGE = "litmuschaos/chaos-exporter:3.0.0"
$env:CONTAINER_RUNTIME_EXECUTOR = "k8sapi"
$env:WORKFLOW_HELPER_IMAGE_VERSION = "3.0.0"

# Langfuse Observability (Cloud)
$env:LANGFUSE_SECRET_KEY = "sk-lf-72694bd7-4a59-430d-b870-0183114c02fe"
$env:LANGFUSE_PUBLIC_KEY = "pk-lf-ba1081a9-7849-427f-8a1c-2a2ee06900c1"
$env:LANGFUSE_HOST = "https://us.cloud.langfuse.com"
$env:LANGFUSE_ORG_ID = "cmlb7dunn00001i06h4p7m8k7"
$env:LANGFUSE_PROJECT_ID = "agentcert"

# Azure OpenAI
$env:AZURE_OPENAI_KEY = "7Sny6BOCWU9JBjOgaEpsPsa4oedY7h5ZqQsgnFSMQhsjRbu8qoM7JQQJ99CAACYeBjFXJ3w3AAABACOGb8ht"
$env:AZURE_OPENAI_ENDPOINT = "https://agentcert.openai.azure.com"
$env:AZURE_OPENAI_DEPLOYMENT = "gpt-4.1-mini"
$env:AZURE_OPENAI_API_VERSION = "2025-01-01-preview"
$env:AZURE_OPENAI_EMBEDDING_DEPLOYMENT = "text-embedding-3-small"

Set-Location "c:\Users\sharmadeep\AgentCert\chaoscenter\graphql\server"
.\server.exe
