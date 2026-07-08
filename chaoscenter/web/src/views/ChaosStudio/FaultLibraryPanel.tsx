import React from 'react';
import { Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useFaultsForApp } from '@api/core';
import type { FaultSpec } from '@api/core';

interface FaultLibraryPanelProps {
  appName: string;
  onFaultDragStart: (faultName: string, evt: React.DragEvent<HTMLDivElement>) => void;
  onFaultClick: (faultName: string) => void;
}

type StepType = 'observe' | 'wait' | 'verify' | 'parallel-fault';

const STEP_TYPE_ICONS: Record<StepType, string> = {
  observe: '👁',
  wait: '⏱',
  verify: '✓',
  'parallel-fault': '⚡'
};

const STEP_TYPE_COLORS: Record<StepType, string> = {
  observe: '#4a90e2',
  wait: '#f39c12',
  verify: '#27ae60',
  'parallel-fault': '#8e44ad'
};

function LibraryItem({
  label,
  subtitle,
  color,
  onDragStart,
  onClick
}: {
  label: string;
  subtitle?: string;
  color: string;
  onDragStart: (e: React.DragEvent<HTMLDivElement>) => void;
  onClick: () => void;
}): React.ReactElement {
  return (
    <div
      draggable
      onDragStart={onDragStart}
      onClick={onClick}
      style={{
        padding: '6px 8px',
        borderRadius: 4,
        cursor: 'grab',
        borderLeft: `3px solid ${color}`,
        background: '#fff',
        marginBottom: 4,
        userSelect: 'none',
        boxShadow: '0 1px 3px rgba(0,0,0,0.08)'
      }}
      title={subtitle ?? label}
    >
      <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_800} lineClamp={1}>
        {label}
      </Text>
      {subtitle && (
        <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_500}>
          {subtitle}
        </Text>
      )}
    </div>
  );
}

function LibraryGroup({
  title,
  titleColor,
  faults,
  onDragStart,
  onClick
}: {
  title: string;
  titleColor: string;
  faults: Array<{ name: string; displayName: string }>;
  onDragStart: (name: string, e: React.DragEvent<HTMLDivElement>) => void;
  onClick: (name: string) => void;
}): React.ReactElement {
  if (faults.length === 0) return <></>;
  return (
    <Layout.Vertical spacing="xsmall">
      <Text
        font={{ variation: FontVariation.SMALL_BOLD }}
        style={{ color: titleColor, textTransform: 'uppercase', letterSpacing: 1 }}
      >
        {title}
      </Text>
      {faults.map(f => (
        <LibraryItem
          key={f.name}
          label={f.displayName ?? f.name}
          subtitle={f.name}
          color="#e74c3c"
          onDragStart={e => onDragStart(f.name, e)}
          onClick={() => onClick(f.name)}
        />
      ))}
    </Layout.Vertical>
  );
}

export default function FaultLibraryPanel({
  appName,
  onFaultDragStart,
  onFaultClick
}: FaultLibraryPanelProps): React.ReactElement {
  const { data, loading } = useFaultsForApp({
    variables: { appName },
    skip: !appName
  });

  const faults: FaultSpec[] = data?.faultsForApp ?? [];
  const generalFaults = faults.filter(f => f.scope === 'GENERAL');
  const domainFaults = faults.filter(f => f.scope === 'DOMAIN');
  const appFaults = faults.filter(f => f.scope === 'APP_SPECIFIC');

  const handleDragStart = (name: string, e: React.DragEvent<HTMLDivElement>) => {
    e.dataTransfer.setData('text/plain', name);
    e.dataTransfer.effectAllowed = 'copy';
    onFaultDragStart(name, e);
  };

  const STEP_TYPES: StepType[] = ['observe', 'wait', 'verify', 'parallel-fault'];

  return (
    <div
      style={{
        width: 220,
        minWidth: 220,
        height: '100%',
        overflowY: 'auto',
        background: '#f8f9fa',
        borderRight: '1px solid #e8eaed',
        padding: 12
      }}
    >
      <Layout.Vertical spacing="large">
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_800}>
            Fault Library
          </Text>
          <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_500}>
            Drag onto canvas or click to add
          </Text>
        </Layout.Vertical>

        {/* Non-fault step types */}
        <Layout.Vertical spacing="xsmall">
          <Text
            font={{ variation: FontVariation.SMALL_BOLD }}
            style={{ color: '#6b7280', textTransform: 'uppercase', letterSpacing: 1 }}
          >
            Step Types
          </Text>
          {STEP_TYPES.map(type => (
            <LibraryItem
              key={type}
              label={`${STEP_TYPE_ICONS[type]} ${type}`}
              color={STEP_TYPE_COLORS[type]}
              onDragStart={e => handleDragStart(`__type:${type}`, e)}
              onClick={() => onFaultClick(`__type:${type}`)}
            />
          ))}
        </Layout.Vertical>

        {loading ? (
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            Loading faults...
          </Text>
        ) : (
          <>
            <LibraryGroup
              title="General"
              titleColor="#6b7280"
              faults={generalFaults}
              onDragStart={handleDragStart}
              onClick={onFaultClick}
            />
            <LibraryGroup
              title={domainFaults[0]?.domain ?? 'Domain'}
              titleColor="#1565c0"
              faults={domainFaults}
              onDragStart={handleDragStart}
              onClick={onFaultClick}
            />
            <LibraryGroup
              title={`${appName} Specific`}
              titleColor="#6a1b9a"
              faults={appFaults}
              onDragStart={handleDragStart}
              onClick={onFaultClick}
            />
          </>
        )}
      </Layout.Vertical>
    </div>
  );
}
