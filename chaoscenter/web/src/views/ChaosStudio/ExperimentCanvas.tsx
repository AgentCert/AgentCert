import React, { useRef, useState } from 'react';
import { Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import type { ExperimentStepDraft } from './wizardTypes';

interface ExperimentCanvasProps {
  steps: ExperimentStepDraft[];
  selectedStepId: string | null;
  onSelectStep: (id: string | null) => void;
  onAddStep: (step: ExperimentStepDraft) => void;
  onRemoveStep: (id: string) => void;
  draggedFaultName: string | null;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function stepLabel(step: ExperimentStepDraft): string {
  switch (step.type) {
    case 'observe':
      return `👁 Observe  ${step.duration ?? '30s'}`;
    case 'wait':
      return `⏱ Wait  ${step.duration ?? '30s'}`;
    case 'fault':
      return `⚡ ${step.faultRef ?? 'fault'}${step.targetMicroservice ? ` → ${step.targetMicroservice}` : ''}`;
    case 'verify':
      return `✓ Verify  ${step.probe?.url ? new URL(step.probe.url).pathname : ''}`;
    case 'parallel-fault':
      return `⚡⚡ Parallel (${(step.parallelFaults ?? []).length} faults)`;
    default:
      return step.name;
  }
}

function stepBorderColor(type: ExperimentStepDraft['type']): string {
  switch (type) {
    case 'observe':
      return '#4a90e2';
    case 'fault':
      return '#e74c3c';
    case 'verify':
      return '#27ae60';
    case 'wait':
      return '#f39c12';
    case 'parallel-fault':
      return '#8e44ad';
    default:
      return '#95a5a6';
  }
}

function buildNewStep(
  draggedFaultName: string,
  existingCount: number
): ExperimentStepDraft {
  const id = `step-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

  if (draggedFaultName.startsWith('__type:')) {
    const type = draggedFaultName.replace('__type:', '') as ExperimentStepDraft['type'];
    return {
      id,
      name: `${type}-${existingCount + 1}`,
      type,
      duration: type === 'observe' || type === 'wait' ? '30s' : undefined
    };
  }

  return {
    id,
    name: `inject-${draggedFaultName}-${existingCount + 1}`,
    type: 'fault',
    faultRef: draggedFaultName,
    targetMicroservice: '',
    params: {}
  };
}

// ── Node component ────────────────────────────────────────────────────────────

function CanvasNode({
  step,
  selected,
  onClick,
  onDelete
}: {
  step: ExperimentStepDraft;
  selected: boolean;
  onClick: () => void;
  onDelete: () => void;
}): React.ReactElement {
  const borderColor = stepBorderColor(step.type);
  return (
    <div
      onClick={onClick}
      style={{
        border: selected ? `2px solid ${borderColor}` : `1px solid #d0d5dd`,
        borderLeft: `4px solid ${borderColor}`,
        borderRadius: 6,
        padding: '8px 12px',
        background: selected ? '#f0f7ff' : '#fff',
        cursor: 'pointer',
        userSelect: 'none',
        boxShadow: selected
          ? `0 0 0 2px ${borderColor}33`
          : '0 1px 4px rgba(0,0,0,0.08)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        minWidth: 280,
        maxWidth: 480,
        width: '100%'
      }}
    >
      <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_800}>
        {stepLabel(step)}
      </Text>
      <button
        onClick={e => {
          e.stopPropagation();
          onDelete();
        }}
        style={{
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          color: '#9aa5b4',
          fontSize: 16,
          padding: '0 4px',
          lineHeight: 1
        }}
        aria-label="Remove step"
        title="Remove step"
      >
        ×
      </button>
    </div>
  );
}

// ── Drop zone component ────────────────────────────────────────────────────────

function DropZone({
  active,
  onDrop,
  onDragOver,
  onDragLeave,
  label
}: {
  active: boolean;
  onDrop: (e: React.DragEvent) => void;
  onDragOver: (e: React.DragEvent) => void;
  onDragLeave: () => void;
  label: string;
}): React.ReactElement {
  return (
    <div
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      style={{
        border: active ? '2px dashed #4a90e2' : '2px dashed #d0d5dd',
        borderRadius: 6,
        padding: '12px 16px',
        textAlign: 'center',
        background: active ? '#e8f0fe' : '#fafafa',
        color: active ? '#1565c0' : '#9aa5b4',
        fontSize: 13,
        cursor: 'copy',
        transition: 'all 0.15s ease',
        minWidth: 280,
        maxWidth: 480,
        width: '100%'
      }}
    >
      {label}
    </div>
  );
}

// ── Main canvas ────────────────────────────────────────────────────────────────

export default function ExperimentCanvas({
  steps,
  selectedStepId,
  onSelectStep,
  onAddStep,
  onRemoveStep,
  draggedFaultName
}: ExperimentCanvasProps): React.ReactElement {
  const [activeDropZone, setActiveDropZone] = useState<number | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const handleDrop = (e: React.DragEvent, _insertIndex: number) => {
    e.preventDefault();
    setActiveDropZone(null);

    const dragged = e.dataTransfer.getData('text/plain') || draggedFaultName;
    if (!dragged) return;
    if (steps.length >= 20) return; // max 20 steps

    const newStep = buildNewStep(dragged, steps.length);
    onAddStep(newStep);
  };

  const handleDragOver = (e: React.DragEvent, idx: number) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
    setActiveDropZone(idx);
  };

  const handleCanvasClick = (e: React.MouseEvent) => {
    // Deselect if clicking canvas background
    if ((e.target as HTMLElement) === containerRef.current) {
      onSelectStep(null);
    }
  };

  return (
    <div
      ref={containerRef}
      onClick={handleCanvasClick}
      style={{
        flex: 1,
        height: '100%',
        overflowY: 'auto',
        background: '#f4f6f8',
        padding: 24,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 0
      }}
    >
      {steps.length === 0 ? (
        <DropZone
          active={activeDropZone === 0}
          onDrop={e => handleDrop(e, 0)}
          onDragOver={e => handleDragOver(e, 0)}
          onDragLeave={() => setActiveDropZone(null)}
          label="Drag a fault here to start building your experiment"
        />
      ) : (
        <>
          {steps.map((step, idx) => (
            <React.Fragment key={step.id}>
              <CanvasNode
                step={step}
                selected={selectedStepId === step.id}
                onClick={() => onSelectStep(step.id)}
                onDelete={() => {
                  if (selectedStepId === step.id) onSelectStep(null);
                  onRemoveStep(step.id);
                }}
              />
              {/* Arrow between nodes */}
              <div
                style={{
                  width: 2,
                  height: 24,
                  background: '#d0d5dd',
                  margin: '0 auto',
                  flexShrink: 0
                }}
              />
              {/* Drop zone between nodes (and after last node) */}
              {idx === steps.length - 1 && (
                <DropZone
                  active={activeDropZone === idx + 1}
                  onDrop={e => handleDrop(e, idx + 1)}
                  onDragOver={e => handleDragOver(e, idx + 1)}
                  onDragLeave={() => setActiveDropZone(null)}
                  label="+ Drop here to add a step"
                />
              )}
            </React.Fragment>
          ))}
          {steps.length >= 20 && (
            <Text font={{ variation: FontVariation.SMALL }} color={Color.YELLOW_700}>
              Maximum of 20 steps reached
            </Text>
          )}
        </>
      )}
    </div>
  );
}
