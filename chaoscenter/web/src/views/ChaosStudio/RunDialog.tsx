import React, { useState } from 'react';
import { Button, ButtonVariation, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';

interface RunDialogProps {
  experimentName: string;
  agentName: string;
  appName: string;
  allowedModels: string[];
  onRun: (selectedModel: string) => void;
  onCancel: () => void;
}

export default function RunDialog({
  experimentName,
  agentName,
  appName,
  allowedModels,
  onRun,
  onCancel
}: RunDialogProps): React.ReactElement {
  const [selectedModel, setSelectedModel] = useState<string>(allowedModels[0] ?? '');

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '6px 10px',
    border: '1px solid #d0d5dd',
    borderRadius: 4,
    fontSize: 14
  };

  const rowStyle: React.CSSProperties = {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '6px 0',
    borderBottom: '1px solid #f0f2f5'
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="run-dialog-title"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0,0,0,0.4)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000
      }}
      onClick={e => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div
        style={{
          background: '#fff',
          borderRadius: 8,
          padding: 32,
          minWidth: 440,
          maxWidth: 560,
          boxShadow: '0 8px 32px rgba(0,0,0,0.18)'
        }}
      >
        <Layout.Vertical spacing="large">
          <Layout.Vertical spacing="xsmall">
            <Text
              id="run-dialog-title"
              font={{ variation: FontVariation.H4 }}
              color={Color.GREY_800}
            >
              Start Experiment Run
            </Text>
            <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
              Review the run details and select a model to proceed
            </Text>
          </Layout.Vertical>

          {/* Run summary */}
          <div style={{ background: '#f8f9fa', borderRadius: 6, padding: 16 }}>
            <div style={rowStyle}>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_600}>
                Experiment
              </Text>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_800}>
                {experimentName}
              </Text>
            </div>
            <div style={rowStyle}>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_600}>
                Agent
              </Text>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_800}>
                {agentName}
              </Text>
            </div>
            <div style={{ ...rowStyle, borderBottom: 'none' }}>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_600}>
                Application
              </Text>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_800}>
                {appName}
              </Text>
            </div>
          </div>

          {/* Model picker */}
          {allowedModels.length > 0 && (
            <div>
              <label
                htmlFor="run-model-select"
                style={{
                  fontSize: 13,
                  fontWeight: 600,
                  color: '#344054',
                  marginBottom: 6,
                  display: 'block'
                }}
              >
                LLM Model
              </label>
              <select
                id="run-model-select"
                value={selectedModel}
                onChange={e => setSelectedModel(e.target.value)}
                style={inputStyle}
              >
                {allowedModels.map(m => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </div>
          )}

          <Layout.Horizontal
            flex={{ justifyContent: 'flex-end' }}
            spacing="medium"
          >
            <Button
              variation={ButtonVariation.TERTIARY}
              text="Cancel"
              onClick={onCancel}
            />
            <Button
              variation={ButtonVariation.PRIMARY}
              text="Start Run →"
              disabled={allowedModels.length > 0 && !selectedModel}
              onClick={() => onRun(selectedModel)}
            />
          </Layout.Horizontal>
        </Layout.Vertical>
      </div>
    </div>
  );
}
