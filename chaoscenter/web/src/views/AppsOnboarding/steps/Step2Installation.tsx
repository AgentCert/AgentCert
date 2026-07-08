import React from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import type { ContributionFormData } from '../types';
import css from '../AppsOnboarding.module.scss';

interface Step2Props {
  data: ContributionFormData;
  onNext: (patch: Partial<ContributionFormData>) => void;
  onBack: () => void;
  onDiscover: (patch: Partial<ContributionFormData>) => Promise<void>;
}

export default function Step2Installation({ data, onNext: _onNext, onBack, onDiscover }: Step2Props): React.ReactElement {
  const [method, setMethod] = React.useState(data.contributeMethod);
  const [chartRepo, setChartRepo] = React.useState(data.chartRepoURL);
  const [chartName, setChartName] = React.useState(data.chartName);
  const [chartVersion, setChartVersion] = React.useState(data.chartVersion);
  const [namespace, setNamespace] = React.useState(data.defaultNamespace);
  const [timeout, setTimeout] = React.useState(data.installTimeout);
  const [discovering, setDiscovering] = React.useState(false);

  const handleDiscover = async (): Promise<void> => {
    setDiscovering(true);
    try {
      await onDiscover({
        contributeMethod: method,
        installMethod: method === 'quick' ? 'external-helm' : 'helm',
        chartRepoURL: chartRepo,
        chartName,
        chartVersion,
        defaultNamespace: namespace,
        installTimeout: timeout,
      });
    } finally {
      setDiscovering(false);
    }
  };

  return (
    <Container className={css.stepContainer}>
      <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800} className={css.stepTitle}>
        Step 2 of 6 — Installation
      </Text>
      <Layout.Vertical spacing="large">
        <Layout.Vertical spacing="medium">
          {(['quick', 'full'] as const).map(m => (
            <div
              key={m}
              className={`${css.methodCard} ${method === m ? css.methodCardSelected : ''}`}
              onClick={() => setMethod(m)}
            >
              <Layout.Vertical spacing="xsmall">
                <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
                  {m === 'quick' ? '🚀 Quick Contribute' : '📦 Full Contribute'}
                </Text>
                <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
                  {m === 'quick'
                    ? "My app already has a public Helm chart. I'll point ACE to it."
                    : "I'll provide K8s manifests or a custom Helm chart."}
                </Text>
              </Layout.Vertical>
            </div>
          ))}
        </Layout.Vertical>

        {method === 'quick' && (
          <Layout.Vertical spacing="medium">
            <div className={css.field}><label className={css.fieldLabel}>Helm Repository URL *</label><input className={css.input} value={chartRepo} onChange={e => setChartRepo(e.target.value)} placeholder="https://charts.example.com" /></div>
            <div className={css.field}><label className={css.fieldLabel}>Chart Name *</label><input className={css.input} value={chartName} onChange={e => setChartName(e.target.value)} placeholder="my-chart" /></div>
            <div className={css.field}><label className={css.fieldLabel}>Chart Version * (pin a specific version)</label><input className={css.input} value={chartVersion} onChange={e => setChartVersion(e.target.value)} placeholder="1.2.3" /><Text font={{ variation: FontVariation.SMALL }} color={Color.ORANGE_700}>⚠ Floating versions (latest, *) are not accepted.</Text></div>
          </Layout.Vertical>
        )}

        {method === 'full' && (
          <Layout.Vertical spacing="medium">
            <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>Provide your chart as a Git repository URL or zip upload.</Text>
            <div className={css.field}><label className={css.fieldLabel}>Git Repository URL</label><input className={css.input} defaultValue={data.gitURL} placeholder="https://github.com/your/chart-repo" /></div>
          </Layout.Vertical>
        )}

        <div className={css.field}><label className={css.fieldLabel}>Default Namespace *</label><input className={css.input} value={namespace} onChange={e => setNamespace(e.target.value)} placeholder="my-app" /></div>
        <div className={css.field}><label className={css.fieldLabel}>Install Timeout</label><input className={css.input} value={timeout} onChange={e => setTimeout(e.target.value)} placeholder="30m" /></div>

        <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
          <Button variation={ButtonVariation.TERTIARY} text="← Back" onClick={onBack} />
          <Button
            variation={ButtonVariation.PRIMARY}
            text={discovering ? 'Discovering...' : 'Discover Services →'}
            disabled={discovering || (method === 'quick' && (!chartRepo || !chartName || !chartVersion))}
            onClick={handleDiscover}
          />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
