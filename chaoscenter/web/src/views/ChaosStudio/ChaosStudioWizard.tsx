import React, { useState } from 'react';
import { Container, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle } from '@hooks';
import { getScope } from '@utils';
import type { ChaosStudioWizardState, ExperimentStepDraft } from './wizardTypes';
import { INITIAL_WIZARD_STATE } from './wizardTypes';
import SelectApp from './SelectApp';
import SelectAgent from './SelectAgent';
import ExperimentCanvas from './ExperimentCanvas';
import FaultLibraryPanel from './FaultLibraryPanel';
import FaultParameterPanel from './FaultParameterPanel';
import ConfigureAndRun from './ConfigureAndRun';

type WizardScreen = 1 | 2 | 3 | 4;

const STEPS = [
  { num: 1, label: 'Select App' },
  { num: 2, label: 'Select Agent' },
  { num: 3, label: 'Build Experiment' },
  { num: 4, label: 'Configure & Run' }
];

function WizardProgress({ current }: { current: WizardScreen }): React.ReactElement {
  return (
    <Layout.Horizontal
      spacing="none"
      flex={{ justifyContent: 'center', alignItems: 'center' }}
      style={{ padding: '0 32px', borderBottom: '1px solid #e8eaed', background: '#fff' }}
    >
      {STEPS.map((step, idx) => (
        <React.Fragment key={step.num}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '12px 16px',
              borderBottom: current === step.num ? '2px solid #4a90e2' : '2px solid transparent',
              opacity: current < step.num ? 0.5 : 1
            }}
          >
            <span
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 22,
                height: 22,
                borderRadius: '50%',
                background:
                  current > step.num
                    ? '#27ae60'
                    : current === step.num
                    ? '#4a90e2'
                    : '#d0d5dd',
                color: '#fff',
                fontSize: 11,
                fontWeight: 700
              }}
            >
              {current > step.num ? '✓' : step.num}
            </span>
            <Text
              font={{ variation: FontVariation.SMALL_BOLD }}
              color={
                current === step.num
                  ? Color.PRIMARY_7
                  : current > step.num
                  ? Color.GREEN_700
                  : Color.GREY_400
              }
            >
              {step.label}
            </Text>
          </div>
          {idx < STEPS.length - 1 && (
            <div style={{ width: 24, height: 1, background: '#e8eaed', margin: '0 4px' }} />
          )}
        </React.Fragment>
      ))}
    </Layout.Horizontal>
  );
}

export default function ChaosStudioWizard(): React.ReactElement {
  const scope = getScope();
  const [screen, setScreen] = useState<WizardScreen>(1);
  const [state, setState] = useState<Partial<ChaosStudioWizardState>>(INITIAL_WIZARD_STATE);
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null);
  const [draggedFault, setDraggedFault] = useState<string | null>(null);

  useDocumentTitle('Chaos Studio — New Experiment');

  const goTo = (n: WizardScreen) => setScreen(n);

  const updateState = (patch: Partial<ChaosStudioWizardState>) =>
    setState(prev => ({ ...prev, ...patch }));

  const addStep = (step: ExperimentStepDraft) =>
    updateState({ steps: [...(state.steps ?? []), step] });

  const removeStep = (id: string) => {
    if (selectedStepId === id) setSelectedStepId(null);
    updateState({ steps: (state.steps ?? []).filter(s => s.id !== id) });
  };

  const updateStep = (id: string, patch: Partial<ExperimentStepDraft>) =>
    updateState({
      steps: (state.steps ?? []).map(s => (s.id === id ? { ...s, ...patch } : s))
    });

  const selectedStep =
    selectedStepId != null
      ? (state.steps ?? []).find(s => s.id === selectedStepId) ?? null
      : null;

  const handleFaultLibraryInteraction = (name: string) => {
    // Click (not drag) in library → append step to canvas
    const existingCount = (state.steps ?? []).length;
    if (existingCount >= 20) return;

    let newStep: ExperimentStepDraft;
    const id = `step-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

    if (name.startsWith('__type:')) {
      const type = name.replace('__type:', '') as ExperimentStepDraft['type'];
      newStep = {
        id,
        name: `${type}-${existingCount + 1}`,
        type,
        duration: type === 'observe' || type === 'wait' ? '30s' : undefined
      };
    } else {
      newStep = {
        id,
        name: `inject-${name}-${existingCount + 1}`,
        type: 'fault',
        faultRef: name,
        targetMicroservice: '',
        params: {}
      };
    }

    addStep(newStep);
    setSelectedStepId(id);
  };

  const renderScreen = () => {
    switch (screen) {
      case 1:
        return (
          <SelectApp
            onSelect={(appName, appDomain, microservices) => {
              updateState({
                selectedAppName: appName,
                selectedAppDomain: appDomain,
                selectedAppMicroservices: microservices
              });
              goTo(2);
            }}
          />
        );

      case 2:
        return (
          <SelectAgent
            appName={state.selectedAppName ?? ''}
            appDomain={state.selectedAppDomain ?? ''}
            onBack={() => goTo(1)}
            onSelect={(agentName, agentVersion, allowUserChoice, allowedModels) => {
              updateState({
                selectedAgentName: agentName,
                selectedAgentVersion: agentVersion,
                selectedAgentAllowUserChoice: allowUserChoice,
                selectedAgentAllowedModels: allowedModels
              });
              goTo(3);
            }}
          />
        );

      case 3:
        return (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 120px)',
              overflow: 'hidden'
            }}
          >
            {/* Context bar */}
            <div
              style={{
                padding: '8px 24px',
                background: '#fff',
                borderBottom: '1px solid #e8eaed',
                display: 'flex',
                alignItems: 'center',
                gap: 16
              }}
            >
              <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
                Step 3 of 4: Build Experiment
              </Text>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                App: <strong>{state.selectedAppName}</strong> &middot; Agent:{' '}
                <strong>{state.selectedAgentName}</strong>
              </Text>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>
                {(state.steps ?? []).length}/20 steps
              </Text>
            </div>

            {/* Three-column layout */}
            <div style={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
              <FaultLibraryPanel
                appName={state.selectedAppName ?? ''}
                onFaultDragStart={name => setDraggedFault(name)}
                onFaultClick={handleFaultLibraryInteraction}
              />
              <ExperimentCanvas
                steps={state.steps ?? []}
                selectedStepId={selectedStepId}
                onSelectStep={setSelectedStepId}
                onAddStep={addStep}
                onRemoveStep={removeStep}
                draggedFaultName={draggedFault}
              />
              <FaultParameterPanel
                step={selectedStep}
                allMicroservices={state.selectedAppMicroservices ?? []}
                onChange={updateStep}
              />
            </div>

            {/* Footer */}
            <div
              style={{
                padding: '12px 24px',
                background: '#fff',
                borderTop: '1px solid #e8eaed',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center'
              }}
            >
              <button
                style={{
                  padding: '8px 16px',
                  border: '1px solid #d0d5dd',
                  borderRadius: 4,
                  background: '#fff',
                  cursor: 'pointer',
                  fontSize: 14
                }}
                onClick={() => goTo(2)}
              >
                ← Back
              </button>
              <button
                style={{
                  padding: '8px 20px',
                  border: 'none',
                  borderRadius: 4,
                  background:
                    (state.steps ?? []).length === 0 ? '#e8eaed' : '#4a90e2',
                  color: (state.steps ?? []).length === 0 ? '#9aa5b4' : '#fff',
                  cursor:
                    (state.steps ?? []).length === 0 ? 'not-allowed' : 'pointer',
                  fontSize: 14,
                  fontWeight: 600
                }}
                disabled={(state.steps ?? []).length === 0}
                onClick={() => goTo(4)}
              >
                Next →
              </button>
            </div>
          </div>
        );

      case 4:
        return (
          <ConfigureAndRun
            wizardState={state}
            steps={state.steps ?? []}
            onBack={() => goTo(3)}
            projectID={scope.projectID}
          />
        );

      default:
        return null;
    }
  };

  // Screen 3 uses its own layout (no DefaultLayoutTemplate wrapper) to maximize canvas space
  if (screen === 3) {
    return (
      <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
        <WizardProgress current={screen} />
        {renderScreen()}
      </div>
    );
  }

  return (
    <DefaultLayoutTemplate
      title="Chaos Studio — New Experiment"
      breadcrumbs={[]}
      noPadding
    >
      <WizardProgress current={screen} />
      <Container style={{ flex: 1, overflowY: 'auto' }}>{renderScreen()}</Container>
    </DefaultLayoutTemplate>
  );
}
