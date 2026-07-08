import React, { useState } from 'react';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useRouteWithBaseUrl } from '@hooks';
import type { ContributionFormData, DiscoveredService } from './types';
import { EMPTY_FORM_DATA, AUTO_EXCLUDE_NAMES, HIGH_CRITICALITY_PATTERNS } from './types';
import Step1Identity from './steps/Step1Identity';
import Step2Installation from './steps/Step2Installation';
import Step3Services from './steps/Step3Services';
import Step4HealthProbe from './steps/Step4HealthProbe';
import Step5LoadTest from './steps/Step5LoadTest';
import Step6Review from './steps/Step6Review';
import css from './AppsOnboarding.module.scss';

export default function AppsOnboardingView(): React.ReactElement {
  const paths = useRouteWithBaseUrl();
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState<ContributionFormData>(EMPTY_FORM_DATA);

  const patch = (data: Partial<ContributionFormData>): void => {
    setFormData(prev => ({ ...prev, ...data }));
  };

  const next = (data: Partial<ContributionFormData>): void => {
    patch(data);
    setStep(s => s + 1);
  };

  const back = (): void => setStep(s => s - 1);

  const handleDiscover = async (data: Partial<ContributionFormData>): Promise<void> => {
    patch(data);

    // Stub — replaced in Stage 11 with real API call
    const stubServices: DiscoveredService[] = [
      { name: 'app-service', label: 'app=app-service', kind: 'deployment', included: true, criticality: 'medium', autoExcluded: false },
      { name: 'app-db', label: 'app=app-db', kind: 'statefulset', included: true, criticality: 'high', autoExcluded: false },
      { name: 'prometheus', label: 'app=prometheus', kind: 'deployment', included: false, criticality: 'low', autoExcluded: true, autoExclusionReason: 'observability tool' },
    ];

    const processed = stubServices.map(svc => {
      let { autoExcluded, autoExclusionReason, criticality, included } = svc;
      if (AUTO_EXCLUDE_NAMES.includes(svc.name)) {
        autoExcluded = true;
        included = false;
        autoExclusionReason = 'observability tool';
      }
      if (HIGH_CRITICALITY_PATTERNS.some(p => p.test(svc.name))) {
        criticality = 'high';
      }
      return { ...svc, autoExcluded, autoExclusionReason, criticality, included };
    });

    patch({ discoveredServices: processed });
    setStep(3);
  };

  const breadcrumbs = [
    { label: 'App Catalog', url: paths.toAppsHub() },
    { label: 'Contribute an App', url: paths.toAppsOnboarding() },
  ];

  const renderStep = (): React.ReactElement => {
    switch (step) {
      case 1: return <Step1Identity data={formData} onNext={next} />;
      case 2: return <Step2Installation data={formData} onNext={next} onBack={back} onDiscover={handleDiscover} />;
      case 3: return <Step3Services data={formData} onNext={next} onBack={back} />;
      case 4: return <Step4HealthProbe data={formData} onNext={next} onBack={back} />;
      case 5: return <Step5LoadTest data={formData} onNext={next} onBack={back} />;
      case 6: return <Step6Review data={formData} onBack={back} />;
      default: return <Step1Identity data={formData} onNext={next} />;
    }
  };

  return (
    <DefaultLayoutTemplate
      title="Contribute an App"
      breadcrumbs={breadcrumbs}
      subTitle="Add a new application to the ACE catalog"
    >
      <div className={css.stepIndicator}>
        {[1, 2, 3, 4, 5, 6].map(n => (
          <div key={n} className={`${css.stepDot} ${step >= n ? css.stepDotActive : ''}`}>
            {n}
          </div>
        ))}
      </div>
      {renderStep()}
    </DefaultLayoutTemplate>
  );
}
