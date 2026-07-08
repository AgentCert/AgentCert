import React from 'react';
import { Container, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useGetFault } from '@api/core';
import type { ExperimentStepDraft } from './wizardTypes';

interface FaultParameterPanelProps {
  step: ExperimentStepDraft | null;
  allMicroservices: string[];
  onChange: (stepId: string, patch: Partial<ExperimentStepDraft>) => void;
}

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '4px 8px',
  border: '1px solid #d0d5dd',
  borderRadius: 4,
  fontSize: 13,
  color: '#344054'
};

const labelStyle: React.CSSProperties = {
  fontSize: 12,
  fontWeight: 600,
  color: '#344054',
  marginBottom: 4,
  display: 'block'
};

export default function FaultParameterPanel({
  step,
  allMicroservices,
  onChange
}: FaultParameterPanelProps): React.ReactElement {
  const { data } = useGetFault({
    variables: { name: step?.faultRef ?? '' },
    skip: !step?.faultRef
  });

  if (!step) {
    return (
      <div
        style={{
          width: 260,
          minWidth: 260,
          height: '100%',
          background: '#f8f9fa',
          borderLeft: '1px solid #e8eaed',
          padding: 16,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center'
        }}
      >
        <Layout.Vertical
          flex={{ justifyContent: 'center', alignItems: 'center' }}
          spacing="small"
        >
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>
            Select a step to configure
          </Text>
        </Layout.Vertical>
      </div>
    );
  }

  const fault = data?.getFault;
  const parameters = fault?.parameters ?? [];

  const updateParam = (key: string, value: string) => {
    onChange(step.id, {
      params: { ...(step.params ?? {}), [key]: value }
    });
  };

  return (
    <div
      style={{
        width: 260,
        minWidth: 260,
        height: '100%',
        overflowY: 'auto',
        background: '#f8f9fa',
        borderLeft: '1px solid #e8eaed',
        padding: 16
      }}
    >
      <Layout.Vertical spacing="medium">
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_800}>
            {fault?.displayName ?? step.name}
          </Text>
          {fault?.description?.short && (
            <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_500}>
              {fault.description.short}
            </Text>
          )}
        </Layout.Vertical>

        {/* Step name */}
        <div>
          <label style={labelStyle}>Step Name</label>
          <input
            style={inputStyle}
            type="text"
            value={step.name}
            onChange={e => onChange(step.id, { name: e.target.value })}
            placeholder="Step name"
          />
        </div>

        {/* Target microservice — for fault steps */}
        {step.type === 'fault' && (
          <div>
            <label style={labelStyle}>
              Target Microservice <span style={{ color: '#e74c3c' }}>*</span>
            </label>
            <select
              style={inputStyle}
              value={step.targetMicroservice ?? ''}
              onChange={e => onChange(step.id, { targetMicroservice: e.target.value })}
            >
              <option value="">-- Select microservice --</option>
              {allMicroservices.map(ms => (
                <option key={ms} value={ms}>
                  {ms}
                </option>
              ))}
            </select>
          </div>
        )}

        {/* Duration — for observe/wait steps */}
        {(step.type === 'observe' || step.type === 'wait') && (
          <div>
            <label style={labelStyle}>Duration</label>
            <input
              style={inputStyle}
              type="text"
              value={step.duration ?? '30s'}
              onChange={e => onChange(step.id, { duration: e.target.value })}
              placeholder="e.g. 30s, 2m"
            />
          </div>
        )}

        {/* Probe — for verify steps */}
        {step.type === 'verify' && (
          <>
            <div>
              <label style={labelStyle}>Probe URL</label>
              <input
                style={inputStyle}
                type="text"
                value={step.probe?.url ?? ''}
                onChange={e =>
                  onChange(step.id, {
                    probe: {
                      ...(step.probe ?? { expectedStatus: 200 }),
                      url: e.target.value
                    }
                  })
                }
                placeholder="http://service.namespace.svc:80/health"
              />
            </div>
            <div>
              <label style={labelStyle}>Expected HTTP Status</label>
              <input
                style={inputStyle}
                type="number"
                value={step.probe?.expectedStatus ?? 200}
                onChange={e =>
                  onChange(step.id, {
                    probe: {
                      ...(step.probe ?? { url: '' }),
                      expectedStatus: parseInt(e.target.value, 10)
                    }
                  })
                }
              />
            </div>
          </>
        )}

        {/* Fault parameters from catalog */}
        {parameters.length > 0 && (
          <Layout.Vertical spacing="small">
            <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_700}>
              Fault Parameters
            </Text>
            {parameters.map(param => (
              <div key={param.key}>
                <label style={labelStyle}>
                  {param.displayName}
                  {param.required && <span style={{ color: '#e74c3c' }}> *</span>}
                </label>
                {param.type === 'BOOLEAN' ? (
                  <input
                    type="checkbox"
                    checked={(step.params?.[param.key] ?? param.default) === 'true'}
                    onChange={e =>
                      updateParam(param.key, e.target.checked ? 'true' : 'false')
                    }
                  />
                ) : param.type === 'ENUM' && param.allowedValues ? (
                  <select
                    style={inputStyle}
                    value={step.params?.[param.key] ?? param.default}
                    onChange={e => updateParam(param.key, e.target.value)}
                  >
                    {param.allowedValues.map(v => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    style={inputStyle}
                    type={
                      param.type === 'INTEGER' || param.type === 'PERCENT'
                        ? 'number'
                        : 'text'
                    }
                    value={step.params?.[param.key] ?? param.default}
                    min={param.min ?? undefined}
                    max={param.max ?? undefined}
                    onChange={e => updateParam(param.key, e.target.value)}
                    placeholder={param.default}
                  />
                )}
                {param.description && (
                  <span style={{ fontSize: 11, color: '#6b7280', display: 'block', marginTop: 2 }}>
                    {param.description}
                    {param.unit ? ` (${param.unit})` : ''}
                  </span>
                )}
              </div>
            ))}
          </Layout.Vertical>
        )}

        {/* Parallel faults config */}
        {step.type === 'parallel-fault' && (
          <Container>
            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
              Parallel fault group: {(step.parallelFaults ?? []).length} faults configured.
              Add fault steps first, then convert them to a parallel group.
            </Text>
          </Container>
        )}
      </Layout.Vertical>
    </div>
  );
}
