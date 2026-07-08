import React, { useState } from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import type { ContributionFormData, LoadTestMethod } from '../types';
import css from '../AppsOnboarding.module.scss';

interface Step5Props {
  data: ContributionFormData;
  onNext: (patch: Partial<ContributionFormData>) => void;
  onBack: () => void;
}

const OPTIONS: { value: LoadTestMethod; title: string; description: string }[] = [
  { value: 'built-in', title: 'My app has a built-in traffic generator', description: '(OTel Demo, apps with otelgen, Fortio)' },
  { value: 'standard', title: "Use ACE's standard load generator", description: 'Image: litmuschaos/litmus-app-deployer:latest' },
  { value: 'custom-job', title: "I'll provide a custom K8s Job for load generation", description: 'Paste your Job YAML below' },
  { value: 'skip', title: 'Skip load test (not recommended)', description: '⚠ Without traffic, most faults produce no observable signal.' },
];

export default function Step5LoadTest({ data, onNext, onBack }: Step5Props): React.ReactElement {
  const [method, setMethod] = useState<LoadTestMethod>(data.loadTestMethod);
  const [jobYAML, setJobYAML] = useState(data.customJobYAML);

  return (
    <Container className={css.stepContainer}>
      <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800} className={css.stepTitle}>
        Step 5 of 6 — Load Test
      </Text>
      <Layout.Vertical spacing="large">
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
          For meaningful chaos results, traffic must be flowing during fault injection.
        </Text>
        <Layout.Vertical spacing="small">
          {OPTIONS.map(opt => (
            <label key={opt.value} className={`${css.radioCard} ${method === opt.value ? css.radioCardSelected : ''}`}>
              <input type="radio" name="loadTestMethod" value={opt.value} checked={method === opt.value} onChange={() => setMethod(opt.value)} className={css.radioInput} />
              <Layout.Vertical spacing="xsmall">
                <Text font={{ variation: FontVariation.BODY1 }} color={Color.GREY_800}>{opt.title}</Text>
                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>{opt.description}</Text>
              </Layout.Vertical>
            </label>
          ))}
        </Layout.Vertical>
        {method === 'custom-job' && (
          <div className={css.field}>
            <label className={css.fieldLabel}>K8s Job YAML</label>
            <textarea className={css.yamlEditor} value={jobYAML} onChange={e => setJobYAML(e.target.value)} placeholder={'apiVersion: batch/v1\nkind: Job\n...'} rows={10} />
          </div>
        )}
        <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
          <Button variation={ButtonVariation.TERTIARY} text="← Back" onClick={onBack} />
          <Button variation={ButtonVariation.PRIMARY} text="Next: Review →" onClick={() => onNext({ loadTestMethod: method, customJobYAML: jobYAML })} />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
