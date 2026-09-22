import React from 'react';
import { useToaster } from '@harnessio/uicore';
import { cloneDeep } from 'lodash-es';
import { replaceHyphen, replaceSpace } from '@utils';
import ExperimentCreationChaosFaultsView from '@views/ExperimentCreationSelectFault/ExperimentCreationChaosFaults';
import { getChaosFaultLazyQuery } from '@api/core';
import type { ChaosHub, Chart } from '@api/entities';
import { useFaultCatalog } from '@hooks';
import type { FaultData } from '@models';

interface ExperimentCreationChaosFaultsControllerProps {
  onSelect: (data: FaultData) => void;
  selectedHub: ChaosHub | undefined;
  chaosCharts: Chart[] | undefined;
  loading: {
    listChaosHub: boolean;
    listChaosFaults: boolean;
  };
  searchParam: string;
  targetApplicationKey?: string;
}

export default function ExperimentCreationChaosFaultsController({
  onSelect,
  selectedHub,
  chaosCharts,
  loading,
  searchParam,
  targetApplicationKey
}: ExperimentCreationChaosFaultsControllerProps): React.ReactElement {
  const { showError } = useToaster();
  const faultCatalog = useFaultCatalog();

  const [getChaosFaultQuery, { loading: getChaosFaultLoading }] = getChaosFaultLazyQuery({
    onError: err => showError(err.message),
    fetchPolicy: 'cache-first'
  });

  const filteredCharts = React.useMemo(() => {
    const deepCopyOfCharts = cloneDeep(chaosCharts);
    const updatedSearchTerm = replaceHyphen(replaceSpace(searchParam)).toLowerCase();
    const filteredChartsWithFilteredExperiments = deepCopyOfCharts?.map(chart => {
      const filteredExperiments = chart.spec.faults.filter(fault => {
        const matchesSearch = replaceHyphen(replaceSpace(fault.name)).toLowerCase().includes(updatedSearchTerm);
        // A fault the catalogue does not know is left listed, so onboarding a
        // fault does not require a catalogue entry before it can be used.
        return matchesSearch && faultCatalog.isFaultCompatible(fault.name, targetApplicationKey);
      });
      return {
        ...chart,
        spec: {
          ...chart.spec,
          faults: filteredExperiments
        }
      };
    });
    return filteredChartsWithFilteredExperiments;
  }, [searchParam, chaosCharts, faultCatalog, targetApplicationKey]);

  return (
    <ExperimentCreationChaosFaultsView
      onSelect={onSelect}
      loading={{ ...loading, getChaosFault: getChaosFaultLoading }}
      selectedHub={selectedHub}
      getChaosFaultQuery={getChaosFaultQuery}
      filteredCharts={filteredCharts}
    />
  );
}
