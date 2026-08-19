import React from 'react';
import { Layout, Switch, Text, VisualYamlSelectedView } from '@harnessio/uicore';
import { Position } from '@blueprintjs/core';
import { Icon } from '@harnessio/icons';
import { Color } from '@harnessio/design-system';
import classNames from 'classnames';
import { useParams } from 'react-router-dom';
import { DiagramFactory } from '@components/PipelineDiagram/DiagramFactory';
import ChaosExperimentNode from '@components/PipelineDiagram/Nodes/ChaosExperimentNode/ChaosExperimentNode';
import CreateNodeStep from '@components/PipelineDiagram/Nodes/CreateNode/CreateNodeStep';
import EndNodeStep from '@components/PipelineDiagram/Nodes/EndNode/EndNodeStep';
import StartNodeStep from '@components/PipelineDiagram/Nodes/StartNode/StartNodeStep';
import { BaseReactComponentProps, NodeType, PipelineGraphState } from '@components/PipelineDiagram/types';
import { Event as DiagramEvent } from '@components/PipelineDiagram/Constants';
import { useStrings } from '@strings';
import ExperimentBuilderTemplateSelectionView from '@views/ExperimentBuilderTemplateSelection';
import type { FaultData, ExperimentManifest, StudioTabs, InfraDetails } from '@models';
import ExperimentCreationSelectFaultController from '@controllers/ExperimentCreationSelectFault';
import ExperimentCreationSelectInstallStepController from '@controllers/ExperimentCreationSelectInstallStep';
import ExperimentAdvancedTuningOptionsView from '@views/ExperimentAdvancedTuningOptions';
import ExperimentCreationTuneFaultView from '@views/ExperimentCreationFaultConfiguration';
import { useUpdateSearchParams, useSearchParams } from '@hooks';
import experimentYamlService from 'services/experiment';
import { GetFaultTunablesOperation } from '@services/experiment/ExperimentYamlService';
import type { ServiceIdentifiers } from '@db';
import type { InfrastructureType } from '@api/entities';
import css from './ExperimentVisualBuilder.module.scss';

interface ExperimentVisualBuilderViewProps {
  setHasFaults: React.Dispatch<React.SetStateAction<boolean>>;
  handleTabChange: (tabID: StudioTabs) => void;
  setViewFilter: (view: VisualYamlSelectedView) => void;
}

export default function ExperimentVisualBuilderView({
  setHasFaults,
  handleTabChange,
  setViewFilter
}: ExperimentVisualBuilderViewProps): React.ReactElement {
  const { getString } = useStrings();
  const searchParams = useSearchParams();
  const updateSearchParams = useUpdateSearchParams();
  const hasUnsavedChangesInURL = searchParams.get('unsavedChanges') === 'true';
  const { experimentKey } = useParams<{ experimentKey: string }>();
  const setUnsavedChanges = (): void => {
    if (!hasUnsavedChangesInURL) updateSearchParams({ unsavedChanges: 'true' });
  };

  const [selectedNodeID, setSelectedNodeID] = React.useState<string>('');
  const [experimentSteps, setExperimentSteps] = React.useState<PipelineGraphState[]>([]);
  const [isSelectFaultDrawerOpen, setIsSelectFaultDrawerOpen] = React.useState<boolean>(false);
  const [isAdvancedExperimentTuningDrawerOpen, setIsAdvancedExperimentTuningDrawerOpen] =
    React.useState<boolean>(false);
  const [tuneFaultDrawerOpen, setTuneFaultDrawerOpen] = React.useState<{
    open: boolean;
    operation: GetFaultTunablesOperation;
  }>({
    open: false,
    operation: GetFaultTunablesOperation.InitialEnvs
  });
  const [isSelectTemplateDrawerOpen, setIsSelectTemplateDrawerOpen] = React.useState<boolean>(false);
  const [installStepDrawer, setInstallStepDrawer] = React.useState<{ open: boolean; kind: 'application' | 'agent' }>({
    open: false,
    kind: 'application'
  });
  const [selectedFaultData, setSelectedFaultData] = React.useState<FaultData>();
  const [serviceIdentifiers, setServiceIdentifiers] = React.useState<ServiceIdentifiers | undefined>();
  const [parallelNodeIdentifier, setParallelNodeIdentifier] = React.useState<string>('');
  const [prevNodeIdentifier, setPrevNodeIdentifier] = React.useState<string>('');
  const [isEditMode, setIsEditMode] = React.useState<boolean>(true);
  const [infraDetails, setInfraDetails] = React.useState<InfraDetails | undefined>();

  const infrastructureType = searchParams.get('infrastructureType') as InfrastructureType | undefined;
  const experimentHandler = experimentYamlService.getInfrastructureTypeHandler(infrastructureType);

  const handleFaultSelection = (faultData: FaultData): void => {
    experimentHandler
      ?.addFaultsToManifest(experimentKey, faultData, parallelNodeIdentifier, prevNodeIdentifier)
      .then(experiment => {
        const steps = experimentHandler.getFaultsFromExperimentManifest(experiment?.manifest, isEditMode);
        setExperimentSteps(steps);

        const hasFaults = experimentHandler.doesExperimentHaveFaults(experiment?.manifest);
        setHasFaults(hasFaults);
        setUnsavedChanges();
      });
    setSelectedFaultData(faultData);
    setIsSelectFaultDrawerOpen(false);
    // Reset state for old fault
    setServiceIdentifiers(undefined);
    if (parallelNodeIdentifier !== '') setParallelNodeIdentifier('');
    if (prevNodeIdentifier !== '') setPrevNodeIdentifier('');
    // Open the tuning drawer
    setTuneFaultDrawerOpen({ open: true, operation: GetFaultTunablesOperation.InitialEnvs });
  };

  const handleInstallStepSelection = (entry: { folder: string; namespace: string }): void => {
    experimentHandler?.addInstallStepToManifest(experimentKey, installStepDrawer.kind, entry).then(experiment => {
      const steps = experimentHandler.getFaultsFromExperimentManifest(experiment?.manifest, isEditMode);
      setExperimentSteps(steps);
      setUnsavedChanges();
    });
    setInstallStepDrawer({ open: false, kind: installStepDrawer.kind });
  };

  const handleSelectTemplateDrawerClose = (manifest: ExperimentManifest, yamlUploaded?: boolean): void => {
    setIsSelectTemplateDrawerOpen(false);
    const steps = experimentHandler?.getFaultsFromExperimentManifest(manifest, isEditMode) ?? [];
    setExperimentSteps(steps);
    const hasFaults = experimentHandler?.doesExperimentHaveFaults(manifest) ?? false;
    setHasFaults(hasFaults);
    if (yamlUploaded) setViewFilter(VisualYamlSelectedView.YAML);
  };

  const handleRemoveFault = (faultName: string): void => {
    experimentHandler?.removeFaultsFromManifest(experimentKey, faultName).then(experiment => {
      const steps = experimentHandler.getFaultsFromExperimentManifest(experiment?.manifest, isEditMode);
      setExperimentSteps(steps);

      const hasFaults = experimentHandler.doesExperimentHaveFaults(experiment?.manifest);
      if (!hasFaults) {
        setHasFaults(hasFaults);
      }
      setUnsavedChanges();
    });
  };

  React.useEffect(() => {
    experimentHandler?.getExperiment(experimentKey).then(experiment => {
      setInfraDetails({
        infraID: experiment?.chaosInfrastructure?.id ?? '',
        environmentID: experiment?.chaosInfrastructure?.environmentID ?? ''
      });

      if (!experiment?.manifest) {
        setIsSelectTemplateDrawerOpen(true);
      }
      const steps = experimentHandler.getFaultsFromExperimentManifest(experiment?.manifest, isEditMode);
      setExperimentSteps(steps);
    });
  }, [isEditMode, experimentKey, experimentHandler]);

  // Initiate DiagramFactory
  const diagram = new DiagramFactory('graph');

  // Register nodes
  diagram.registerNode('ChaosNode', ChaosExperimentNode as React.FC<BaseReactComponentProps>, true);
  diagram.registerNode(NodeType.EndNode, EndNodeStep);
  diagram.registerNode(NodeType.StartNode, StartNodeStep);
  diagram.registerNode(NodeType.CreateNode, CreateNodeStep as unknown as React.FC<BaseReactComponentProps>);

  const ChaosDiagram = diagram.render();

  const eventListeners = {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    [DiagramEvent.ClickNode]: (event: any) => {
      setSelectedNodeID(event.data.id);
      experimentHandler?.getExperiment(experimentKey).then(experiment => {
        const faultData = experimentHandler.getFaultData(experiment?.manifest, event.data.id);
        setSelectedFaultData(faultData);
        // If edit mode is enabled, drawer will be open
        if (isEditMode) {
          setTuneFaultDrawerOpen({ open: true, operation: GetFaultTunablesOperation.UpdatedEnvs });
        }
      });
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    [DiagramEvent.AddLinkClicked]: (event: any) => {
      if (event.data.prevNodeIdentifier !== undefined) setPrevNodeIdentifier(event.data.prevNodeIdentifier);
      setIsSelectFaultDrawerOpen(true);
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    [DiagramEvent.AddParallelNode]: (event: any) => {
      setParallelNodeIdentifier(event.data.identifier);
      setIsSelectFaultDrawerOpen(true);
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    [DiagramEvent.RemoveNode]: (event: any) => {
      handleRemoveFault(event.data.identifier);
    }
  };
  diagram.registerListeners(eventListeners);

  // Fault nodes whose ChaosEngine has no appns/applabel configured -- see
  // KubernetesYamlService.getFaultsFromExperimentManifest, which computes
  // isInComplete per node. Surfaced as a single list here (rather than only
  // the per-node warning badge) so a multi-fault experiment doesn't require
  // clicking through every node to discover which ones won't inject
  // against anything.
  const faultsMissingTarget = experimentSteps
    .flatMap(step => [step, ...(step.children ?? [])])
    .filter(step => step.data?.isInComplete)
    .map(step => step.name);

  return (
    <div className={css.graphContainer}>
      <Layout.Vertical className={css.options} spacing="small">
        <Switch
          checked={!isEditMode}
          label={getString('preview')}
          onChange={() => setIsEditMode(!isEditMode)}
          className={isEditMode ? classNames(css.preview, css.greyText) : classNames(css.preview, css.activeText)}
        />

        <div className={css.divider} />

        <Text className={css.groupLabel}>{getString('setup')}</Text>
        <Layout.Vertical spacing="xsmall">
          <Text
            icon="chaos-scenario-builder"
            iconProps={{ name: 'chaos-scenario-builder', size: 16 }}
            onClick={() => setInstallStepDrawer({ open: true, kind: 'application' })}
            className={css.actionItem}
            tooltip={getString('installApplicationDescription')}
            tooltipProps={{ position: Position.RIGHT }}
          >
            {getString('installApplication')}
          </Text>

          <Text
            icon="chaos-scenario-builder"
            iconProps={{ name: 'chaos-scenario-builder', size: 16 }}
            onClick={() => setInstallStepDrawer({ open: true, kind: 'agent' })}
            className={css.actionItem}
            tooltip={getString('installAgentDescription')}
            tooltipProps={{ position: Position.RIGHT }}
          >
            {getString('installAgent')}
          </Text>
        </Layout.Vertical>

        <div className={css.divider} />

        <Text className={css.groupLabel}>{getString('configuration')}</Text>
        <Text
          icon="ci-build-pipeline"
          iconProps={{ name: 'ci-build-pipeline', size: 16 }}
          onClick={() => setIsAdvancedExperimentTuningDrawerOpen(true)}
          className={css.actionItem}
          tooltip={getString('advancedOptionsDescription')}
          tooltipProps={{ position: Position.RIGHT }}
        >
          {getString('advancedOptions')}
        </Text>
      </Layout.Vertical>
      {isAdvancedExperimentTuningDrawerOpen && (
        <ExperimentAdvancedTuningOptionsView
          isOpen={isAdvancedExperimentTuningDrawerOpen}
          onClose={() => setIsAdvancedExperimentTuningDrawerOpen(false)}
        />
      )}
      {isSelectTemplateDrawerOpen && (
        <ExperimentBuilderTemplateSelectionView
          isSelectTemplateDrawerOpen={isSelectTemplateDrawerOpen}
          onClose={handleSelectTemplateDrawerClose}
          handleTabChange={handleTabChange}
        />
      )}
      {isSelectFaultDrawerOpen && (
        <ExperimentCreationSelectFaultController
          isOpen={isSelectFaultDrawerOpen}
          onSelect={handleFaultSelection}
          onClose={() => setIsSelectFaultDrawerOpen(false)}
        />
      )}
      {installStepDrawer.open && (
        <ExperimentCreationSelectInstallStepController
          isOpen={installStepDrawer.open}
          kind={installStepDrawer.kind}
          onSelect={handleInstallStepSelection}
          onClose={() => setInstallStepDrawer({ open: false, kind: installStepDrawer.kind })}
        />
      )}
      {tuneFaultDrawerOpen.open && (
        <ExperimentCreationTuneFaultView
          isOpen={tuneFaultDrawerOpen.open}
          onClose={() => setTuneFaultDrawerOpen({ open: false, operation: GetFaultTunablesOperation.UpdatedEnvs })}
          initialFaultData={selectedFaultData}
          infraID={infraDetails?.infraID}
          environmentID={infraDetails?.environmentID}
          faultTuneOperation={tuneFaultDrawerOpen.operation}
          initialServiceIdentifiers={serviceIdentifiers}
        />
      )}
      {faultsMissingTarget.length > 0 && (
        <Layout.Horizontal className={css.missingTargetBanner} spacing="small" flex={{ alignItems: 'center' }}>
          <Icon name="warning-sign" size={14} color="orange500" />
          <Text font={{ size: 'small' }} color={Color.ORANGE_900}>
            {getString('faultsMissingTargetApplication', { faultNames: faultsMissingTarget.join(', ') })}
          </Text>
        </Layout.Horizontal>
      )}
      <ChaosDiagram
        data={experimentSteps}
        selectedNodeId={selectedNodeID}
        readonly={!isEditMode}
        createNodeTitle={getString('add')}
      />
    </div>
  );
}
