import React, { useState } from 'react';
import { Button, ButtonVariation, Container, Layout, Text, useToaster } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useHistory } from 'react-router-dom';
import { useCreateExperiment, useSubmitRun } from '@api/core';
import { useRouteWithBaseUrl } from '@hooks';
import type { ChaosStudioWizardState, ExperimentStepDraft } from './wizardTypes';
import RunDialog from './RunDialog';

interface ConfigureAndRunProps {
  wizardState: Partial<ChaosStudioWizardState>;
  steps: ExperimentStepDraft[];
  onBack: () => void;
  projectID: string;
}

type ModelMode = 'agent-default' | 'fixed' | 'user-chooses-at-run';

interface PerStepCriteria {
  stepName: string;
  detectWithinSecs: number;
  mitigateWithinSecs: number;
}

const DEFAULT_METRICS = [
  'time_to_detect',
  'time_to_mitigate',
  'tool_call_efficiency',
  'root_cause_accuracy',
  'false_positive_rate',
  'remediation_correctness'
];

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '6px 10px',
  border: '1px solid #d0d5dd',
  borderRadius: 4,
  fontSize: 14,
  color: '#344054'
};

const labelStyle: React.CSSProperties = {
  fontSize: 13,
  fontWeight: 600,
  color: '#344054',
  marginBottom: 6,
  display: 'block'
};

const sectionStyle: React.CSSProperties = {
  background: '#fff',
  border: '1px solid #e8eaed',
  borderRadius: 8,
  padding: 20
};

export default function ConfigureAndRun({
  wizardState,
  steps,
  onBack,
  projectID
}: ConfigureAndRunProps): React.ReactElement {
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const { showSuccess, showError } = useToaster();

  const appName = wizardState.selectedAppName ?? '';
  const agentName = wizardState.selectedAgentName ?? '';
  const allowUserChoice = wizardState.selectedAgentAllowUserChoice ?? false;
  const allowedModels = wizardState.selectedAgentAllowedModels ?? [];

  const [experimentName, setExperimentName] = useState<string>(
    `${appName}-${Date.now()}`
  );
  const [hypothesis, setHypothesis] = useState<string>(
    wizardState.hypothesis ?? ''
  );
  const [tags, setTags] = useState<string>('');
  const [modelMode, setModelMode] = useState<ModelMode>('agent-default');
  const [fixedModel, setFixedModel] = useState<string>('');
  const [agentSecrets, setAgentSecrets] = useState<Record<string, string>>({});
  const [perStepCriteria, setPerStepCriteria] = useState<PerStepCriteria[]>(
    steps
      .filter(s => s.type === 'fault')
      .map(s => ({
        stepName: s.name,
        detectWithinSecs: 60,
        mitigateWithinSecs: 120
      }))
  );
  const [selectedMetrics, setSelectedMetrics] = useState<Set<string>>(
    new Set(DEFAULT_METRICS)
  );
  const [showRunDialog, setShowRunDialog] = useState<boolean>(false);

  const [createExperiment, { loading: creating }] = useCreateExperiment();
  const [submitRun, { loading: submitting }] = useSubmitRun();

  const buildExperimentInput = () => ({
    name: experimentName,
    hypothesis: hypothesis || undefined,
    tags: tags
      ? tags
          .split(',')
          .map(t => t.trim())
          .filter(Boolean)
      : [],
    targetApp: { name: appName, version: '>=1.0.0' },
    agentConstraints: { supportedAgents: [agentName] },
    modelSelection: {
      mode:
        modelMode === 'agent-default'
          ? 'AGENT_DEFAULT'
          : modelMode === 'fixed'
          ? 'FIXED'
          : 'USER_CHOOSES_AT_RUN',
      fixedModel: modelMode === 'fixed' ? fixedModel : undefined
    },
    steps: steps.map(s => ({
      name: s.name,
      type: s.type.toUpperCase().replace(/-/g, '_'),
      duration: s.duration,
      faultRef: s.faultRef,
      target: s.targetMicroservice
        ? { microservice: s.targetMicroservice }
        : undefined,
      params: s.params
        ? Object.entries(s.params).map(([key, value]) => ({ key, value }))
        : [],
      probe: s.probe
    })),
    successCriteria:
      perStepCriteria.length > 0 ? { perStep: perStepCriteria } : undefined,
    evaluationMetrics: Array.from(selectedMetrics)
  });

  const doSaveExperiment = async (): Promise<boolean> => {
    try {
      await createExperiment({
        variables: { projectID, input: buildExperimentInput() }
      });
      return true;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to save experiment';
      showError(message);
      return false;
    }
  };

  const doSubmitRun = async (modelOverride?: string) => {
    const secrets = Object.entries(agentSecrets).map(([key, value]) => ({ key, value }));
    try {
      const { data } = await submitRun({
        variables: {
          projectID,
          experimentName,
          agentName,
          modelOverride: modelOverride || undefined,
          secretOverrides: secrets.length > 0 ? secrets : undefined
        }
      });
      const runID = data?.submitRun?.runID;
      if (runID) {
        showSuccess(`Run submitted: ${runID}`);
        history.push(paths.toChaosStudioRun({ runID }));
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to submit run';
      showError(message);
    }
  };

  const handleSaveDraft = async () => {
    const ok = await doSaveExperiment();
    if (ok) showSuccess(`Experiment "${experimentName}" saved as draft`);
  };

  const handleSaveAndRun = async () => {
    const ok = await doSaveExperiment();
    if (!ok) return;

    if (allowUserChoice && modelMode === 'user-chooses-at-run') {
      setShowRunDialog(true);
    } else {
      await doSubmitRun(modelMode === 'fixed' ? fixedModel : undefined);
    }
  };

  const toggleMetric = (m: string) => {
    setSelectedMetrics(prev => {
      const next = new Set(prev);
      if (next.has(m)) next.delete(m);
      else next.add(m);
      return next;
    });
  };

  const updateCriteria = (
    idx: number,
    field: 'detectWithinSecs' | 'mitigateWithinSecs',
    value: number
  ) => {
    setPerStepCriteria(prev => {
      const next = [...prev];
      next[idx] = { ...next[idx], [field]: value };
      return next;
    });
  };

  return (
    <Container padding="xlarge">
      <Layout.Vertical spacing="large">
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
            Step 4 of 4: Configure &amp; Run
          </Text>
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
            App: <strong>{appName}</strong> &middot; Agent:{' '}
            <strong>{agentName}</strong> &middot; {steps.length} step
            {steps.length !== 1 ? 's' : ''}
          </Text>
        </Layout.Vertical>

        {/* Experiment name */}
        <div style={sectionStyle}>
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
            Experiment Details
          </Text>
          <Layout.Vertical spacing="medium" style={{ marginTop: 12 }}>
            <div>
              <label style={labelStyle}>
                Experiment Name <span style={{ color: '#e74c3c' }}>*</span>
              </label>
              <input
                style={inputStyle}
                type="text"
                value={experimentName}
                onChange={e => setExperimentName(e.target.value)}
                placeholder="e.g. sock-shop-pod-failure-cascade"
              />
            </div>
            <div>
              <label style={labelStyle}>Tags (comma-separated)</label>
              <input
                style={inputStyle}
                type="text"
                value={tags}
                onChange={e => setTags(e.target.value)}
                placeholder="e.g. regression, nightly, pod-failure"
              />
            </div>
            <div>
              <label style={labelStyle}>Hypothesis</label>
              <textarea
                style={{ ...inputStyle, minHeight: 80, resize: 'vertical' }}
                value={hypothesis}
                onChange={e => setHypothesis(e.target.value)}
                placeholder="Describe what you expect the agent to detect and mitigate..."
              />
            </div>
          </Layout.Vertical>
        </div>

        {/* Model selection */}
        {allowUserChoice && (
          <div style={sectionStyle}>
            <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
              LLM Model
            </Text>
            <Layout.Vertical spacing="small" style={{ marginTop: 12 }}>
              {(['agent-default', 'fixed', 'user-chooses-at-run'] as ModelMode[]).map(mode => (
                <label key={mode} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <input
                    type="radio"
                    name="modelMode"
                    value={mode}
                    checked={modelMode === mode}
                    onChange={() => setModelMode(mode)}
                  />
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                    {mode === 'agent-default'
                      ? 'Use agent default model'
                      : mode === 'fixed'
                      ? 'Fix model for this experiment'
                      : 'User chooses at run time'}
                  </Text>
                </label>
              ))}
              {modelMode === 'fixed' && allowedModels.length > 0 && (
                <select
                  style={{ ...inputStyle, marginLeft: 24, width: 'calc(100% - 24px)' }}
                  value={fixedModel}
                  onChange={e => setFixedModel(e.target.value)}
                >
                  <option value="">Select model</option>
                  {allowedModels.map(m => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                </select>
              )}
            </Layout.Vertical>
          </div>
        )}

        {/* Agent secrets */}
        <div style={sectionStyle}>
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
            Agent Secrets
          </Text>
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} style={{ marginTop: 4 }}>
            Provide runtime secrets for <strong>{agentName}</strong>. These are stored
            securely per-run and never persisted.
          </Text>
          <Layout.Vertical spacing="small" style={{ marginTop: 12 }}>
            {['API_KEY', 'LANGFUSE_SECRET_KEY'].map(secretKey => (
              <div key={secretKey}>
                <label style={labelStyle}>{secretKey}</label>
                <input
                  style={inputStyle}
                  type="password"
                  value={agentSecrets[secretKey] ?? ''}
                  onChange={e =>
                    setAgentSecrets(prev => ({ ...prev, [secretKey]: e.target.value }))
                  }
                  placeholder="••••••••••••••••"
                />
              </div>
            ))}
          </Layout.Vertical>
        </div>

        {/* Success criteria */}
        {perStepCriteria.length > 0 && (
          <div style={sectionStyle}>
            <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
              Success Criteria
            </Text>
            <div style={{ overflowX: 'auto', marginTop: 12 }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                <thead>
                  <tr style={{ background: '#f8f9fa' }}>
                    <th style={{ textAlign: 'left', padding: '8px 12px', fontWeight: 600 }}>
                      Step
                    </th>
                    <th style={{ textAlign: 'left', padding: '8px 12px', fontWeight: 600 }}>
                      Detect within (s)
                    </th>
                    <th style={{ textAlign: 'left', padding: '8px 12px', fontWeight: 600 }}>
                      Mitigate within (s)
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {perStepCriteria.map((c, i) => (
                    <tr key={c.stepName} style={{ borderTop: '1px solid #f0f2f5' }}>
                      <td style={{ padding: '8px 12px' }}>{c.stepName}</td>
                      <td style={{ padding: '8px 12px' }}>
                        <input
                          type="number"
                          value={c.detectWithinSecs}
                          onChange={e =>
                            updateCriteria(i, 'detectWithinSecs', parseInt(e.target.value, 10))
                          }
                          style={{ ...inputStyle, width: 80 }}
                          min={1}
                        />
                      </td>
                      <td style={{ padding: '8px 12px' }}>
                        <input
                          type="number"
                          value={c.mitigateWithinSecs}
                          onChange={e =>
                            updateCriteria(i, 'mitigateWithinSecs', parseInt(e.target.value, 10))
                          }
                          style={{ ...inputStyle, width: 80 }}
                          min={1}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Evaluation metrics */}
        <div style={sectionStyle}>
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
            Evaluation Metrics
          </Text>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
              gap: 8,
              marginTop: 12
            }}
          >
            {DEFAULT_METRICS.map(m => (
              <label key={m} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input
                  type="checkbox"
                  checked={selectedMetrics.has(m)}
                  onChange={() => toggleMetric(m)}
                />
                <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                  {m.replace(/_/g, ' ')}
                </Text>
              </label>
            ))}
          </div>
        </div>

        {/* Action buttons */}
        <Layout.Horizontal
          flex={{ justifyContent: 'space-between', alignItems: 'center' }}
          padding={{ top: 'medium' }}
        >
          <Button
            variation={ButtonVariation.TERTIARY}
            icon="chevron-left"
            text="Back"
            onClick={onBack}
          />
          <Layout.Horizontal spacing="medium">
            <Button
              variation={ButtonVariation.SECONDARY}
              text="Save as Draft"
              disabled={creating || !experimentName}
              onClick={handleSaveDraft}
            />
            <Button
              variation={ButtonVariation.PRIMARY}
              text={submitting ? 'Submitting...' : 'Save & Run →'}
              disabled={creating || submitting || !experimentName || steps.length === 0}
              onClick={handleSaveAndRun}
            />
          </Layout.Horizontal>
        </Layout.Horizontal>
      </Layout.Vertical>

      {/* Run dialog for user-chooses-at-run */}
      {showRunDialog && (
        <RunDialog
          experimentName={experimentName}
          agentName={agentName}
          appName={appName}
          allowedModels={allowedModels}
          onRun={model => {
            setShowRunDialog(false);
            doSubmitRun(model);
          }}
          onCancel={() => setShowRunDialog(false)}
        />
      )}
    </Container>
  );
}
