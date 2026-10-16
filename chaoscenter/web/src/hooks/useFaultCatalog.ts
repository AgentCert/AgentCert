import React from 'react';
import { getFaultCatalog } from '@api/core/faultCatalog/getFaultCatalog';
import type { FaultCatalog, TargetApplication } from '@api/entities';
// useAppStore directly rather than getScope from '@utils': getScope itself
// imports the '@hooks' barrel, so reaching it from a hook is a cycle.
import { useAppStore } from './useAppStore';

/**
 * Resolved view over the server's fault catalogue.
 *
 * Every predicate here fails OPEN, because the catalogue records known
 * restrictions rather than enumerating what is permitted: a fault, agent, or
 * application the catalogue has not heard of stays usable. That is what keeps
 * onboarding a new one a single chart edit, and it means a catalogue that fails
 * to load degrades the builder to its old unfiltered behaviour instead of
 * blocking it entirely. The server-side validator is the authoritative gate.
 */
export interface FaultCatalogResolver {
  loading: boolean;
  applications: TargetApplication[];
  /** The application an install-application step's `-folder=`/`-namespace=` refers to. */
  resolveApplication: (folder: string | undefined, namespace: string | undefined) => TargetApplication | undefined;
  /** Whether a fault may target an application key. */
  isFaultCompatible: (faultName: string | undefined, appKey: string | undefined) => boolean;
  /** Kubernetes kinds a fault acts on; undefined means unrestricted. */
  faultWorkloadKinds: (faultName: string | undefined) => string[] | undefined;
  /** Services a fault hardcodes; undefined means any service of the app. */
  faultRequiredServices: (faultName: string | undefined) => string[] | undefined;
  /** `<app>/<service>` pairs known to fail this fault. */
  faultKnownFailingTargets: (faultName: string | undefined) => string[];
}

const normalize = (value: string | undefined): string => value?.trim().toLowerCase() ?? '';

function buildResolver(catalog: FaultCatalog | undefined, loading: boolean): FaultCatalogResolver {
  const applications = catalog?.applications ?? [];

  const byFolder = new Map<string, TargetApplication>();
  const byNamespace = new Map<string, TargetApplication>();
  for (const app of applications) {
    for (const folder of app.folders) {
      byFolder.set(normalize(folder), app);
    }
    const namespace = normalize(app.namespace);
    if (namespace && !byNamespace.has(namespace)) {
      byNamespace.set(namespace, app);
    }
  }

  const byFault = new Map((catalog?.faults ?? []).map(fault => [fault.faultName, fault]));

  return {
    loading,
    applications,
    // The folder wins: an operator may override the namespace, but not the
    // chart the step installs.
    resolveApplication: (folder, namespace) =>
      byFolder.get(normalize(folder)) ?? byNamespace.get(normalize(namespace)),
    isFaultCompatible: (faultName, appKey) => {
      if (!faultName || !appKey) return true;
      const fault = byFault.get(faultName);
      if (!fault) return true;
      return fault.compatibleApps.includes(appKey);
    },
    faultWorkloadKinds: faultName => {
      const kinds = faultName ? byFault.get(faultName)?.workloadKinds : undefined;
      return kinds && kinds.length > 0 ? kinds : undefined;
    },
    faultRequiredServices: faultName => {
      const services = faultName ? byFault.get(faultName)?.requiredServices : undefined;
      return services && services.length > 0 ? services : undefined;
    },
    faultKnownFailingTargets: faultName => (faultName ? byFault.get(faultName)?.knownFailingTargets ?? [] : [])
  };
}

/**
 * An agent's declared application restriction.
 *
 * `null`/`undefined` means the agent declared none and works with every
 * application, including ones onboarded after it. `[]` means it is deliberately
 * not application-targeted.
 */
export function isAgentCompatible(declared: string[] | null | undefined, appKey: string | undefined): boolean {
  if (declared === null || declared === undefined) return true;
  if (!appKey) return declared.length > 0;
  return declared.includes(appKey);
}

export function useFaultCatalog(): FaultCatalogResolver {
  const { projectID } = useAppStore();
  // A catalogue read failure must not break the builder, so the error is
  // swallowed here and the resolver degrades to unfiltered.
  const { data, loading } = getFaultCatalog({
    projectID: projectID ?? '',
    options: { onError: () => undefined }
  });

  return React.useMemo(() => buildResolver(data?.getFaultCatalog, loading), [data, loading]);
}
