import React, { useState } from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import type { ContributionFormData } from '../types';
import css from '../AppsOnboarding.module.scss';

interface Step4Props {
  data: ContributionFormData;
  onNext: (patch: Partial<ContributionFormData>) => void;
  onBack: () => void;
}

export default function Step4HealthProbe({ data, onNext, onBack }: Step4Props): React.ReactElement {
  const [url, setUrl] = useState(data.healthProbeURL);
  const [status, setStatus] = useState(data.healthProbeStatus);
  const [delay, setDelay] = useState(data.initialDelaySeconds);
  const [period, setPeriod] = useState(data.periodSeconds);
  const [threshold, setThreshold] = useState(data.failureThreshold);
  const [urlError, setUrlError] = useState('');

  const totalTimeout = delay + period * threshold;

  const handleNext = (): void => {
    if (!url.includes('{{.AppNamespace}}')) {
      setUrlError('URL must use {{.AppNamespace}} instead of a hardcoded namespace');
      return;
    }
    setUrlError('');
    onNext({ healthProbeURL: url, healthProbeStatus: status, initialDelaySeconds: delay, periodSeconds: period, failureThreshold: threshold });
  };

  return (
    <Container className={css.stepContainer}>
      <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800} className={css.stepTitle}>
        Step 4 of 6 — Health Probe
      </Text>
      <Layout.Vertical spacing="large">
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
          ACE will probe this URL after install to confirm the app is ready before running any faults.
        </Text>
        <div className={css.field}>
          <label className={css.fieldLabel}>Health Probe URL *</label>
          <input className={`${css.input} ${urlError ? css.inputError : ''}`} value={url} onChange={e => { setUrl(e.target.value); setUrlError(''); }} placeholder="http://my-service.{{.AppNamespace}}.svc.cluster.local:80/health" />
          {urlError ? <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>{urlError}</Text> : <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>ⓘ Use {'{{.AppNamespace}}'} instead of the literal namespace.</Text>}
        </div>
        <div className={css.field}>
          <label className={css.fieldLabel}>Expected HTTP Status *</label>
          <input className={css.input} value={status} onChange={e => setStatus(e.target.value)} placeholder="200" style={{ maxWidth: 100 }} />
        </div>
        <Layout.Horizontal spacing="large">
          <div className={css.field}><label className={css.fieldLabel}>Initial Delay (s)</label><input type="number" className={css.input} value={delay} onChange={e => setDelay(Number(e.target.value))} style={{ maxWidth: 100 }} /></div>
          <div className={css.field}><label className={css.fieldLabel}>Retry Interval (s)</label><input type="number" className={css.input} value={period} onChange={e => setPeriod(Number(e.target.value))} style={{ maxWidth: 100 }} /></div>
          <div className={css.field}><label className={css.fieldLabel}>Max Retries</label><input type="number" className={css.input} value={threshold} onChange={e => setThreshold(Number(e.target.value))} style={{ maxWidth: 100 }} /></div>
        </Layout.Horizontal>
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
          Total timeout: {delay} + ({period} × {threshold}) = {totalTimeout} seconds
        </Text>
        <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
          <Button variation={ButtonVariation.TERTIARY} text="← Back" onClick={onBack} />
          <Button variation={ButtonVariation.PRIMARY} text="Next: Load Test →" onClick={handleNext} />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
