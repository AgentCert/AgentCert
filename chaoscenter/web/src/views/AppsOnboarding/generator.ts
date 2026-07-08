import type { ContributionFormData } from './types';

export function generateAppYAML(data: ContributionFormData): string {
  const selectedServices = data.discoveredServices.filter(s => s.included);

  const microservicesYAML = selectedServices.map(svc =>
`    - name: ${svc.name}
      displayName: ${svc.name}
      k8s:
        label: "${svc.label}"
        kind: ${svc.kind}
        namespace: "{{.AppNamespace}}"
      criticality: ${svc.criticality}
      relevantFaults: [pod-delete, pod-cpu-hog]
      dependsOn: []`
  ).join('\n');

  const installSection = data.installMethod === 'external-helm'
    ? `  install:
    method: external-helm
    chartRef:
      repo: "${data.chartRepoURL}"
      chart: "${data.chartName}"
      version: "${data.chartVersion}"
    namespace:
      default: ${data.defaultNamespace || 'my-app'}
      configurable: false
    timeout: ${data.installTimeout}
    wait: true`
    : `  install:
    method: helm
    folder: ${data.name}
    namespace:
      default: ${data.defaultNamespace || 'my-app'}
      configurable: false
    timeout: ${data.installTimeout}
    wait: true`;

  const loadTestSection = (() => {
    switch (data.loadTestMethod) {
      case 'built-in': return `  loadTest:\n    enabled: false\n    method: external`;
      case 'standard': return `  loadTest:\n    enabled: true\n    method: deployer\n    image: litmuschaos/litmus-app-deployer:latest\n    args: ["-namespace=loadtest", "-app=loadtest"]`;
      case 'custom-job': return `  loadTest:\n    enabled: true\n    method: job\n    jobSpec: {}`;
      case 'skip': return `  loadTest:\n    enabled: false`;
    }
  })();

  return `apiVersion: ace.io/v1
kind: AppCatalogEntry
metadata:
  name: ${data.name}
  displayName: "${data.displayName}"
  version: "1.0.0"
  tier: community
  domain: ${data.domain}
  capabilityDomains: [${data.domain}, common]
  tags: []
  maintainers:
    - name: ${data.maintainerName}
      email: ${data.maintainerEmail}

spec:
  description:
    short: "${data.shortDescription}"
    long: |
      ${data.longDescription.split('\n').join('\n      ')}
    suitableFor: []
    notSuitableFor: []

${installSection}

  healthProbe:
    url: "${data.healthProbeURL}"
    expectedStatus: "${data.healthProbeStatus}"
    initialDelaySeconds: ${data.initialDelaySeconds}
    periodSeconds: ${data.periodSeconds}
    failureThreshold: ${data.failureThreshold}

${loadTestSection}

  microservices:
${microservicesYAML}

  faultCompatibility:
    - faultName: pod-delete
      compatible: true
      notes: "Adjust based on your app's actual fault behavior"
      recommendedTargets: [${selectedServices.slice(0, 2).map(s => s.name).join(', ')}]

  rbac:
    chaosRunnerPermissions:
      - apiGroups: [""]
        resources: [pods, events, "pods/exec", "pods/log"]
        verbs: [get, list, watch, delete, create]
      - apiGroups: [apps]
        resources: [deployments, replicasets, statefulsets]
        verbs: [get, list, watch, patch]
      - apiGroups: [litmuschaos.io]
        resources: [chaosengines, chaosexperiments, chaosresults]
        verbs: [get, list, create, update, patch, delete, watch]
      - apiGroups: ["batch"]
        resources: [jobs]
        verbs: [get, list, create, delete, watch]
`;
}

export function generateReadmeMD(data: ContributionFormData): string {
  const selectedServices = data.discoveredServices.filter(s => s.included);
  return `# ${data.displayName}

**Domain:** ${data.domain}
**Version:** 1.0.0
**Tier:** Community
**Maintainer:** ${data.maintainerName}

## Overview

${data.longDescription}

## Microservices

| Service | K8s Label | Kind | Criticality |
|---------|----------|------|-------------|
${selectedServices.map(s => `| ${s.name} | ${s.label} | ${s.kind} | ${s.criticality} |`).join('\n')}

## Install

\`\`\`bash
helm install ${data.name} catalog/apps/community/${data.name}/chart \\
  --namespace ${data.defaultNamespace} --create-namespace --timeout ${data.installTimeout} --wait
\`\`\`

Health probe: \`${data.healthProbeURL}\` → expects HTTP ${data.healthProbeStatus}.
`;
}

export function downloadFilesAsZip(appYAML: string, readmeMD: string, appName: string): void {
  downloadFile(`${appName}-app.yaml`, appYAML);
  window.setTimeout(() => downloadFile(`${appName}-README.md`, readmeMD), 300);
}

function downloadFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'text/plain' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
