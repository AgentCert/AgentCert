import React from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Container, Layout, Text } from '@harnessio/uicore';
import type { ContributionFormData, DiscoveredService } from '../types';
import css from '../AppsOnboarding.module.scss';

interface Step3Props {
  data: ContributionFormData;
  onNext: (patch: Partial<ContributionFormData>) => void;
  onBack: () => void;
}

export default function Step3Services({ data, onNext, onBack }: Step3Props): React.ReactElement {
  const [services, setServices] = React.useState<DiscoveredService[]>(data.discoveredServices);

  const toggleIncluded = (name: string): void => {
    setServices(prev => prev.map(s => s.name === name ? { ...s, included: !s.included } : s));
  };

  const setCriticality = (name: string, criticality: DiscoveredService['criticality']): void => {
    setServices(prev => prev.map(s => s.name === name ? { ...s, criticality } : s));
  };

  return (
    <Container className={css.stepContainer}>
      <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800} className={css.stepTitle}>
        Step 3 of 6 — Services & Fault Targets
      </Text>
      <Layout.Vertical spacing="large">
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
          We discovered {services.length} services in your chart. Review and confirm which ones should be available as fault targets.
        </Text>

        {services.length === 0 ? (
          <Text font={{ variation: FontVariation.BODY }} color={Color.ORANGE_500}>
            No services discovered. Go back and check your chart reference.
          </Text>
        ) : (
          <div className={css.serviceTable}>
            <div className={`${css.serviceRow} ${css.serviceHeader}`}>
              <span>Service Name</span><span>K8s Label</span><span>Kind</span><span>Criticality</span><span>Include?</span>
            </div>
            {services.map(svc => (
              <div key={svc.name} className={`${css.serviceRow} ${svc.autoExcluded ? css.autoExcludedRow : ''}`}>
                <span>
                  {svc.name}
                  {svc.autoExcluded && <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}> (auto-excluded: {svc.autoExclusionReason})</Text>}
                </span>
                <span><code>{svc.label}</code></span>
                <span>{svc.kind}</span>
                <span>
                  <select value={svc.criticality} onChange={e => setCriticality(svc.name, e.target.value as DiscoveredService['criticality'])} disabled={!svc.included} className={css.criticalitySelect}>
                    <option value="high">high</option>
                    <option value="medium">medium</option>
                    <option value="low">low</option>
                  </select>
                </span>
                <span><input type="checkbox" checked={svc.included} onChange={() => toggleIncluded(svc.name)} /></span>
              </div>
            ))}
          </div>
        )}

        {services.some(s => s.autoExcluded) && (
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            ⓘ Prometheus and Grafana are excluded by default — faulting observability tools breaks the experiment.
          </Text>
        )}

        <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
          <Button variation={ButtonVariation.TERTIARY} text="← Back" onClick={onBack} />
          <Button
            variation={ButtonVariation.PRIMARY}
            text="Next: Health Probe →"
            disabled={services.filter(s => s.included).length === 0}
            onClick={() => onNext({ discoveredServices: services })}
          />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
