import React, { useEffect, useState } from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import type { ContributionFormData } from '../types';
import { generateAppYAML, generateReadmeMD, downloadFilesAsZip } from '../generator';
import css from '../AppsOnboarding.module.scss';

interface Step6Props {
  data: ContributionFormData;
  onBack: () => void;
}

export default function Step6Review({ data, onBack }: Step6Props): React.ReactElement {
  const [appYAML, setAppYAML] = useState('');
  const [readmeMD, setReadmeMD] = useState('');
  const [showPreview, setShowPreview] = useState(false);

  useEffect(() => {
    setAppYAML(generateAppYAML(data));
    setReadmeMD(generateReadmeMD(data));
  }, [data]);

  const selectedServices = data.discoveredServices.filter(s => s.included);

  return (
    <Container className={css.stepContainer}>
      <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800} className={css.stepTitle}>
        Step 6 of 6 — Review & Generate
      </Text>
      <Layout.Vertical spacing="large">
        <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_700}>Your app spec summary:</Text>
        <div className={css.summaryGrid}>
          <div className={css.summaryRow}><span>Name:</span><strong>{data.name}</strong></div>
          <div className={css.summaryRow}><span>Domain:</span><strong>{data.domain}</strong></div>
          <div className={css.summaryRow}><span>Tier:</span><strong>Community (pending review)</strong></div>
          <div className={css.summaryRow}><span>Services:</span><strong>{selectedServices.length} services for fault targeting</strong></div>
          <div className={css.summaryRow}><span>Load Test:</span><strong>{data.loadTestMethod}</strong></div>
        </div>

        <Layout.Vertical spacing="small">
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>ACE will generate these files:</Text>
          <div className={css.fileList}>
            <div className={css.fileItem}>✅ catalog/apps/community/{data.name}/app.yaml</div>
            <div className={css.fileItem}>✅ catalog/apps/community/{data.name}/docs/README.md</div>
          </div>
        </Layout.Vertical>

        {showPreview && (
          <Layout.Vertical spacing="small">
            <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>app.yaml preview:</Text>
            <pre className={css.yamlPreview}>{appYAML}</pre>
          </Layout.Vertical>
        )}

        <Button
          variation={ButtonVariation.LINK}
          text={showPreview ? 'Hide preview' : 'Preview app.yaml'}
          onClick={() => setShowPreview(v => !v)}
        />

        <Layout.Vertical spacing="small">
          <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>What happens next:</Text>
          <ol className={css.nextSteps}>
            <li>Download the generated files</li>
            <li>Open a PR to the ACE monorepo</li>
            <li>CI runs schema validation + helm lint</li>
            <li>Community review (typically 2–3 business days)</li>
            <li>Merge → app live in catalog</li>
          </ol>
        </Layout.Vertical>

        <Layout.Horizontal spacing="medium" flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Button variation={ButtonVariation.TERTIARY} text="← Back" onClick={onBack} />
          <Button
            variation={ButtonVariation.PRIMARY}
            text="Download Files"
            icon="download"
            onClick={() => downloadFilesAsZip(appYAML, readmeMD, data.name)}
          />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
