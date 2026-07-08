import React, { useState } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, Dialog, useToaster } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Step1Identity } from './Step1Identity';
import { Step2DockerImage } from './Step2DockerImage';
import { Step3LLMConfig } from './Step3LLMConfig';
import { Step4ConfigInputs } from './Step4ConfigInputs';
import { Step5CapabilitiesTools } from './Step5CapabilitiesTools';
import { Step6AppCompatibility } from './Step6AppCompatibility';
import { Step7Review } from './Step7Review';
import { initialWizardState } from './types';
import type { WizardState } from './types';
import { getScope } from '@utils';
import { useRegisterAgent } from '@api/core';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onRegistered: () => void;
}

const STEPS = [
  { label: 'Identity' },
  { label: 'Image' },
  { label: 'LLM' },
  { label: 'Inputs' },
  { label: 'Capabilities' },
  { label: 'Compatibility' },
  { label: 'Review' },
];

export const AgentRegistrationWizard: React.FC<Props> = ({ isOpen, onClose, onRegistered }) => {
  const { projectID } = getScope();
  const { showSuccess, showError } = useToaster();
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>(initialWizardState);
  const [registerAgent, { loading: registering }] = useRegisterAgent();

  const update = (updates: Partial<WizardState>) => setState(prev => ({ ...prev, ...updates }));

  const visibleSteps = STEPS.filter((_, idx) => {
    // Skip LLM step (index 2) when agent is not LLM-dependent
    if (idx === 2 && !state.llmDependent) return false;
    return true;
  });

  const totalSteps = visibleSteps.length;
  const currentLabel = visibleSteps[step]?.label ?? '';

  const goNext = () => { if (step < totalSteps - 1) setStep(s => s + 1); };
  const goBack = () => { if (step > 0) setStep(s => s - 1); };

  const handleRegister = async () => {
    try {
      await registerAgent({
        projectID,
        request: {
          name: state.agentName,
          description: state.shortDescription || undefined,
          namespace: 'litmus',
          capabilities: state.capabilities,
          version: 'v1.0.0',
        },
      });
      showSuccess('Agent registered successfully!');
      setState(initialWizardState);
      setStep(0);
      onRegistered();
      onClose();
    } catch (e: unknown) {
      showError((e as Error).message);
    }
  };

  const renderStep = () => {
    switch (currentLabel) {
      case 'Identity': return <Step1Identity state={state} onChange={update} />;
      case 'Image': return <Step2DockerImage state={state} onChange={update} />;
      case 'LLM': return <Step3LLMConfig state={state} onChange={update} projectID={projectID} />;
      case 'Inputs': return <Step4ConfigInputs state={state} onChange={update} />;
      case 'Capabilities': return <Step5CapabilitiesTools state={state} onChange={update} />;
      case 'Compatibility': return <Step6AppCompatibility state={state} onChange={update} />;
      case 'Review': return <Step7Review state={state} onChange={update} />;
      default: return null;
    }
  };

  return (
    <Dialog isOpen={isOpen} onClose={onClose} title="Register Agent" style={{ width: 720, maxHeight: '90vh' }}>
      <Layout.Vertical style={{ height: '100%' }}>
        {/* Step indicators */}
        <Container style={{ borderBottom: '1px solid #E0E0E0', padding: '8px 24px' }}>
          <Layout.Horizontal spacing="small" style={{ alignItems: 'center' }}>
            {visibleSteps.map((s, idx) => (
              <React.Fragment key={s.label}>
                <Container
                  style={{
                    width: 28, height: 28, borderRadius: '50%',
                    background: idx <= step ? '#0278D5' : '#E0E0E0',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    cursor: 'pointer',
                  }}
                  onClick={() => setStep(idx)}
                >
                  <Text color={Color.WHITE} font={{ variation: FontVariation.TINY_SEMI }}>
                    {idx + 1}
                  </Text>
                </Container>
                <Text font={{ variation: FontVariation.TINY }} color={idx === step ? Color.PRIMARY_7 : Color.GREY_600}>
                  {s.label}
                </Text>
                {idx < visibleSteps.length - 1 && (
                  <div style={{ flex: 1, height: 1, background: '#E0E0E0', margin: '0 4px' }} />
                )}
              </React.Fragment>
            ))}
          </Layout.Horizontal>
        </Container>

        {/* Step content */}
        <Container style={{ flex: 1, overflow: 'auto', padding: 24 }}>
          {renderStep()}
        </Container>

        {/* Footer */}
        <Container style={{ borderTop: '1px solid #E0E0E0', padding: '12px 24px' }}>
          <Layout.Horizontal spacing="small" style={{ justifyContent: 'space-between' }}>
            <Button
              text="Back"
              variation={ButtonVariation.TERTIARY}
              disabled={step === 0}
              onClick={goBack}
            />
            {step < totalSteps - 1 ? (
              <Button text="Next" variation={ButtonVariation.PRIMARY} onClick={goNext} />
            ) : (
              <Button
                text={registering ? 'Registering...' : 'Register Agent'}
                variation={ButtonVariation.PRIMARY}
                loading={registering}
                onClick={handleRegister}
              />
            )}
          </Layout.Horizontal>
        </Container>
      </Layout.Vertical>
    </Dialog>
  );
};
