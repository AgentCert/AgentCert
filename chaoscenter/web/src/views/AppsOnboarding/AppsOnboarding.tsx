import React, { useState } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2 } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import css from './AppsOnboarding.module.scss';

export enum OnboardingMethod {
  KUBERNETES_MANIFEST = 'kubernetes-manifest',
  DOCKER_IMAGE = 'docker-image',
  HELM_CHART = 'helm-chart',
  CLOUD_MANAGED = 'cloud-managed'
}

interface RadioOption {
  value: OnboardingMethod;
  title: string;
  description: string;
}

interface Application {
  id: string;
  name: string;
  method: string;
  status: string;
  createdAt: string;
}

// Mock data for applications table
const mockApplications: Application[] = [
  { id: '1', name: 'Payment Service', method: 'Kubernetes Manifest', status: 'Active', createdAt: '2026-01-18' },
  { id: '2', name: 'User Auth API', method: 'Docker Image', status: 'Active', createdAt: '2026-01-12' },
  { id: '3', name: 'Notification Service', method: 'Helm Chart', status: 'Inactive', createdAt: '2026-01-08' },
  { id: '4', name: 'Analytics Dashboard', method: 'Cloud Managed', status: 'Active', createdAt: '2026-01-03' }
];

export default function AppsOnboardingView(): React.ReactElement {
  const { getString } = useStrings();
  const { showSuccess } = useToaster();
  const location = useLocation();
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const searchParams = new URLSearchParams(location.search);
  const showOptions = searchParams.get('step') === 'select';
  const [selectedMethod, setSelectedMethod] = useState<OnboardingMethod | null>(null);
  const [applications, setApplications] = useState<Application[]>(mockApplications);

  useDocumentTitle(getString('appsOnboarding'));

  const breadcrumbs = [
    {
      label: getString('appsOnboarding'),
      url: paths.toAppsOnboarding()
    }
  ];

  const getMethodLabel = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.KUBERNETES_MANIFEST:
        return getString('onboardAppUsingKubernetesManifest');
      case OnboardingMethod.DOCKER_IMAGE:
        return getString('onboardAppUsingDockerImage');
      case OnboardingMethod.HELM_CHART:
        return getString('onboardAppUsingHelmChart');
      case OnboardingMethod.CLOUD_MANAGED:
        return getString('onboardAppUsingCloudManaged');
      default:
        return method;
    }
  };

  const radioOptions: RadioOption[] = [
    {
      value: OnboardingMethod.KUBERNETES_MANIFEST,
      title: getString('onboardAppUsingKubernetesManifest'),
      description: getString('onboardAppUsingKubernetesManifestDesc')
    },
    {
      value: OnboardingMethod.DOCKER_IMAGE,
      title: getString('onboardAppUsingDockerImage'),
      description: getString('onboardAppUsingDockerImageDesc')
    },
    {
      value: OnboardingMethod.HELM_CHART,
      title: getString('onboardAppUsingHelmChart'),
      description: getString('onboardAppUsingHelmChartDesc')
    },
    {
      value: OnboardingMethod.CLOUD_MANAGED,
      title: getString('onboardAppUsingCloudManaged'),
      description: getString('onboardAppUsingCloudManagedDesc')
    }
  ];

  const handleOnboard = (): void => {
    if (selectedMethod) {
      showSuccess(`You have selected: ${getMethodLabel(selectedMethod)}`);
    }
  };

  const handleNewApplication = (): void => {
    history.push({ search: '?step=select' });
  };

  const handleBack = (): void => {
    history.push({ search: '' });
  };

  const handleEditApplication = (app: Application): void => {
    showSuccess(`Editing application: ${app.name}`);
  };

  const handleDeleteApplication = (app: Application): void => {
    setApplications(prev => prev.filter(a => a.id !== app.id));
    showSuccess(`Deleted application: ${app.name}`);
  };

  const columns: Column<Application>[] = [
    {
      Header: getString('name'),
      accessor: 'name',
      Cell: ({ value }: CellProps<Application>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_900}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('method'),
      accessor: 'method',
      Cell: ({ value }: CellProps<Application>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('status'),
      accessor: 'status',
      Cell: ({ value }: CellProps<Application>) => (
        <Text 
          font={{ variation: FontVariation.BODY }} 
          color={value === 'Active' ? Color.GREEN_700 : Color.GREY_500}
        >
          {value}
        </Text>
      )
    },
    {
      Header: getString('createdAt'),
      accessor: 'createdAt',
      Cell: ({ value }: CellProps<Application>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('actions'),
      id: 'actions',
      Cell: ({ row }: CellProps<Application>) => (
        <Layout.Horizontal spacing="medium">
          <Text
            className={css.actionLink}
            color={Color.PRIMARY_7}
            onClick={() => handleEditApplication(row.original)}
          >
            {getString('edit')}
          </Text>
          <Text
            className={css.actionLink}
            color={Color.RED_600}
            onClick={() => handleDeleteApplication(row.original)}
          >
            {getString('delete')}
          </Text>
        </Layout.Horizontal>
      )
    }
  ];

  return (
    <DefaultLayoutTemplate
      breadcrumbs={breadcrumbs}
      title={getString('appsOnboarding')}
    >
      <Container className={css.container}>
        {!showOptions ? (
          <Layout.Vertical spacing="large">
            <Text
              font={{ variation: FontVariation.H3 }}
              color={Color.GREY_800}
              className={css.heading}
            >
              {getString('appsOnboarding')}
            </Text>
            <Container className={css.newAppButtonContainer}>
              <Button
                variation={ButtonVariation.PRIMARY}
                text={getString('newApplication')}
                icon="plus"
                onClick={handleNewApplication}
              />
            </Container>

            {applications.length > 0 && (
              <Container className={css.tableContainer}>
                <Text
                  font={{ variation: FontVariation.H5 }}
                  color={Color.GREY_800}
                  className={css.tableHeading}
                >
                  {getString('onboardedApplications')}
                </Text>
                <TableV2<Application>
                  columns={columns}
                  data={applications}
                  className={css.appsTable}
                />
              </Container>
            )}
          </Layout.Vertical>
        ) : (
          <Layout.Vertical spacing="large">
            <Text
              font={{ variation: FontVariation.H3 }}
              color={Color.GREY_800}
              className={css.heading}
            >
              {getString('onboardYourApp')}
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
                variation={ButtonVariation.TERTIARY}
                text={getString('back')}
                icon="arrow-left"
                onClick={handleBack}
              />
              <Button
                variation={ButtonVariation.PRIMARY}
                text={getString('onboard')}
                onClick={handleOnboard}
                disabled={!selectedMethod}
              />
            </Container>
          </Layout.Vertical>
        )}
      </Container>
    </DefaultLayoutTemplate>
  );
}
