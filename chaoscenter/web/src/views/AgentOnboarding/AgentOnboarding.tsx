import React, { useState } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle } from '@hooks';
import { useStrings } from '@strings';
import css from './AgentOnboarding.module.scss';

export enum OnboardingMethod {
  HELM_CHART = 'helm-chart',
  APIS = 'apis',
  FAAS = 'faas'
}

interface RadioOption {
  value: OnboardingMethod;
  title: string;
  description: string;
}

export default function AgentOnboardingView(): React.ReactElement {
  const { getString } = useStrings();
  const { showSuccess } = useToaster();
  const [selectedMethod, setSelectedMethod] = useState<OnboardingMethod | null>(null);

  useDocumentTitle(getString('agentOnboarding'));

  const getMethodLabel = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return getString('onboardUsingHelmChart');
      case OnboardingMethod.APIS:
        return getString('onboardUsingAPIs');
      case OnboardingMethod.FAAS:
        return getString('onboardUsingFaaS');
      default:
        return method;
    }
  };

  const radioOptions: RadioOption[] = [
    {
      value: OnboardingMethod.HELM_CHART,
      title: getString('onboardUsingHelmChart'),
      description: getString('onboardUsingHelmChartDesc')
    },
    {
      value: OnboardingMethod.APIS,
      title: getString('onboardUsingAPIs'),
      description: getString('onboardUsingAPIsDesc')
    },
    {
      value: OnboardingMethod.FAAS,
      title: getString('onboardUsingFaaS'),
      description: getString('onboardUsingFaaSDesc')
    }
  ];

  const handleOnboard = (): void => {
    if (selectedMethod) {
      showSuccess(`You have selected: ${getMethodLabel(selectedMethod)}`);
    }
  };

  return (
    <DefaultLayoutTemplate
      breadcrumbs={[]}
      title={getString('agentOnboarding')}
    >
      <Container className={css.container}>
        <Layout.Vertical spacing="large">
          <Text
            font={{ variation: FontVariation.H3 }}
            color={Color.GREY_800}
            className={css.heading}
          >
            {getString('onboardYourAgent')}
          </Text>

          <div className={css.radioGroup}>
            {radioOptions.map(option => (
              <label
                key={option.value}
                className={cx(css.radioCard, {
                  [css.selected]: selectedMethod === option.value
                })}
              >
                <input
                  type="radio"
                  name="onboardingMethod"
                  value={option.value}
                  checked={selectedMethod === option.value}
                  onChange={() => setSelectedMethod(option.value)}
                  className={css.radioInput}
                />
                <div className={css.radioContent}>
                  <Text font={{ variation: FontVariation.BODY1 }} color={Color.GREY_900} className={css.radioTitle}>
                    {option.title}
                  </Text>
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} className={css.radioDescription}>
                    {option.description}
                  </Text>
                </div>
              </label>
            ))}
          </div>

          <Container className={css.buttonContainer}>
            <Button
              variation={ButtonVariation.PRIMARY}
              text={getString('onboard')}
              onClick={handleOnboard}
              disabled={!selectedMethod}
            />
          </Container>
        </Layout.Vertical>
      </Container>
    </DefaultLayoutTemplate>
  );
}
