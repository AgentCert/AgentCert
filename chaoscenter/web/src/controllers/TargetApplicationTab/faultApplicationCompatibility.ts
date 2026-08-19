// Machine-readable mirror of agents/FAULT_APPLICATION_COMPATIBILITY.md at the
// monorepo root. That file is compiled by reading each fault's engine.yaml,
// ChaosExperiment description, ground_truth.yaml and the pre-baked Argo
// Workflow experiment.yaml templates -- it is the authoritative source. Keep
// this file in sync whenever that one changes.
//
// Used by the Target Application tab (see TargetApplicationTab.tsx, the
// controller in this directory) to narrow the AppKind/AppNamespace/AppLabel
// pickers to values that are actually meaningful for the fault being
// configured, instead of offering every live namespace/object indiscriminately.
// A fault with no entry here is treated as fully unrestricted -- the pickers
// fall back to today's behavior (every live/pending namespace, every gvrData
// kind, every object found in the selected namespace).

export type CompatibleApp = 'otel-demo' | 'sock-shop' | 'book-info';

export const APP_NAMESPACES: Record<CompatibleApp, string> = {
  'otel-demo': 'otel-demo',
  'sock-shop': 'sock-shop',
  'book-info': 'book-info'
};

// "Notable services" per app, from the compatibility doc's summary table.
// This is the default target list for faults that are generically
// compatible with an app (i.e. don't hardcode one specific service).
export const APP_SERVICES: Record<CompatibleApp, string[]> = {
  'otel-demo': [
    'accounting',
    'ad',
    'cart',
    'checkout',
    'currency',
    'email',
    'flagd',
    'fraud-detection',
    'frontend',
    'image-provider',
    'kafka',
    'payment',
    'product-catalog',
    'quote',
    'recommendation',
    'shipping',
    'valkey-cart'
  ],
  'sock-shop': [
    'carts',
    'carts-db',
    'catalogue',
    'catalogue-db',
    'orders',
    'orders-db',
    'payment',
    'queue-master',
    'rabbitmq',
    'shipping',
    'user',
    'user-db'
  ],
  'book-info': ['details', 'reviews', 'ratings', 'productpage']
};

export interface FaultCompatibility {
  // Apps this fault can legitimately target.
  apps: CompatibleApp[];
  // Kubernetes kinds this fault actually operates on -- filters the AppKind
  // dropdown to this list when present. Omit when the fault doesn't select
  // pods via a Deployment/StatefulSet workload (node-level faults, or ones
  // that patch a ConfigMap/Service directly) -- the AppKind picker then
  // stays fully unrestricted rather than guessing wrong.
  appKinds?: string[];
  // Per-app override of the target service list, for faults that only make
  // sense against one named service (the OpenTelemetry-Demo-exclusive faults
  // in the compatibility doc's §2b). Falls back to APP_SERVICES[app] when
  // omitted for an app this fault is compatible with.
  servicesByApp?: Partial<Record<CompatibleApp, string[]>>;
}

const ALL_APPS: CompatibleApp[] = ['otel-demo', 'sock-shop', 'book-info'];
const WORKLOAD_KINDS = ['deployment', 'statefulset'];

// §1 standard LitmusChaos faults (chaos-charts/faults/kubernetes/) --
// app-agnostic pod/network/L7/config-level ones. Node-level faults
// (node-cpu-hog, node-drain, kubelet-service-kill, etc.) target a nodeLabel,
// not appns/applabel, so they have no entry here.
const STANDARD_GENERIC: string[] = [
  'container-kill',
  'pod-cpu-hog',
  'pod-cpu-hog-exec',
  'pod-memory-hog',
  'pod-memory-hog-exec',
  'pod-io-stress',
  'disk-fill',
  'pod-autoscaler',
  'pod-delete',
  'pod-network-loss',
  'pod-network-latency',
  'pod-network-corruption',
  'pod-network-duplication',
  'pod-network-partition',
  'pod-network-rate-limit',
  'pod-dns-error',
  'pod-dns-spoof',
  'pod-http-latency',
  'pod-http-status-code',
  'pod-http-modify-body',
  'pod-http-modify-header',
  'pod-http-reset-peer',
  'misconfigured-kubernetes-workload-container-readiness-probe',
  'modified-target-port-kubernetes-service',
  'invalid-kubernetes-service-selector',
  'nonexistent-kubernetes-workload-persistent-volume-claim'
];

// §2a ITBench generic faults -- same "blank appns/applabel for the caller to
// fill in" mechanism, mechanically compatible with all three apps.
const ITBENCH_GENERIC: string[] = [
  'modified-kubernetes-workload-container-environment-variable',
  'nonexistent-kubernetes-workload-container-image',
  'cordoned-kubernetes-worker-node',
  'failing-name-resolution-kubernetes-workload-dns-policy',
  'insufficient-kubernetes-workload-container-resources',
  'scaled-to-zero-kubernetes-workload',
  'invalid-kubernetes-workload-container-command',
  'deleted-kubernetes-service',
  'crashing-kubernetes-workload-init-container',
  'hanging-kubernetes-workload-init-container',
  'ingress-port-blocking-network-policy',
  'insufficient-kubernetes-resource-quota',
  'kubernetes-api-server-request-surge',
  'misconfigured-kubernetes-horizontal-pod-autoscaler',
  'nonexistent-kubernetes-workload-node',
  'priority-kubernetes-workload-priority-preemption',
  'unassigned-kubernetes-workload-container-resource-limits',
  'unschedulable-kubernetes-workload-pod-anti-affinity-rule',
  'unsupported-architecture-kubernetes-workload-container-image'
];

// Faults in the generic lists above that don't select pods by
// Deployment/StatefulSet in the usual way, so AppKind stays unrestricted.
const NO_APPKIND_RESTRICTION = new Set([
  'cordoned-kubernetes-worker-node',
  'ingress-port-blocking-network-policy',
  'insufficient-kubernetes-resource-quota',
  'kubernetes-api-server-request-surge',
  'misconfigured-kubernetes-horizontal-pod-autoscaler',
  'deleted-kubernetes-service',
  'invalid-kubernetes-service-selector',
  'modified-target-port-kubernetes-service'
]);

const FAULT_COMPATIBILITY: Record<string, FaultCompatibility> = {};

for (const fault of [...STANDARD_GENERIC, ...ITBENCH_GENERIC]) {
  FAULT_COMPATIBILITY[fault] = {
    apps: ALL_APPS,
    appKinds: NO_APPKIND_RESTRICTION.has(fault) ? undefined : WORKLOAD_KINDS
  };
}

// §2b OpenTelemetry Demo-exclusive faults -- hardcode a specific otel-demo
// service by name, so they cannot be pointed at Sock Shop or Bookinfo.
FAULT_COMPATIBILITY['chaos-mesh-pod-failure-replacement'] = {
  apps: ['otel-demo'],
  appKinds: WORKLOAD_KINDS,
  servicesByApp: { 'otel-demo': ['checkout'] }
};
FAULT_COMPATIBILITY['chaos-mesh-http-body-tamper-replacement'] = {
  apps: ['otel-demo'],
  appKinds: WORKLOAD_KINDS,
  servicesByApp: { 'otel-demo': ['email'] }
};
FAULT_COMPATIBILITY['chaos-mesh-http-abort-replacement'] = {
  apps: ['otel-demo'],
  appKinds: WORKLOAD_KINDS,
  servicesByApp: { 'otel-demo': ['quote'] }
};
FAULT_COMPATIBILITY['valkey-workload-changed-password'] = {
  apps: ['otel-demo'],
  appKinds: WORKLOAD_KINDS,
  servicesByApp: { 'otel-demo': ['valkey-cart'] }
};
FAULT_COMPATIBILITY['valkey-workload-out-of-memory'] = {
  apps: ['otel-demo'],
  appKinds: WORKLOAD_KINDS,
  servicesByApp: { 'otel-demo': ['valkey-cart'] }
};
// Patches the flagd-config ConfigMap directly -- not a Deployment/StatefulSet
// label-selector target, so AppKind stays unrestricted.
FAULT_COMPATIBILITY['opentelemetry-demo-feature-flag'] = {
  apps: ['otel-demo'],
  servicesByApp: { 'otel-demo': ['flagd'] }
};

export function getFaultCompatibility(faultName: string | undefined): FaultCompatibility | undefined {
  if (!faultName) return undefined;
  return FAULT_COMPATIBILITY[faultName];
}
