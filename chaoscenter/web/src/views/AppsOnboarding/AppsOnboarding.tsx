import React, { useState, useRef } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2, TextInput, DropDown, SelectOption } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import { getScope } from '@utils';
import { listEnvironment } from '@api/core/environments';
import type { Environment } from '@api/entities';
import css from './AppsOnboarding.module.scss';

export enum OnboardingMethod {
  HELM_CHART = 'helm-chart',
  CLOUD_MANAGED = 'cloud-managed'
}

interface RadioOption {
  value: OnboardingMethod;
  title: string;
  description: string;
}

interface UploadedFile {
  name: string;
  method: OnboardingMethod;
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
  const [uploadedFile, setUploadedFile] = useState<UploadedFile | null>(null);
  const [cloudManagedUrl, setCloudManagedUrl] = useState<string>('');
  const [selectedEnvironment, setSelectedEnvironment] = useState<string>('');
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [applications, setApplications] = useState<Application[]>(mockApplications);
  const scope = getScope();

  // Fetch environments list
  const { data: environmentsData, loading: environmentsLoading, error: environmentsError } = listEnvironment({
    projectID: scope.projectID,
    environmentIDs: [],
    pagination: { page: 0, limit: 100 },
    options: {
      fetchPolicy: 'cache-and-network'
    }
  });

  const environments: Environment[] = environmentsData?.listEnvironments?.environments || [];
  const environmentOptions: SelectOption[] = environments.map(env => ({
    label: env.name,
    value: env.environmentID
  }));

  // Debug logging
  React.useEffect(() => {
    console.log('Environments Debug:', {
      projectID: scope.projectID,
      loading: environmentsLoading,
      error: environmentsError,
      data: environmentsData,
      environments: environments.length,
      options: environmentOptions.length
    });
  }, [environmentsData, environmentsLoading, environmentsError]);

  useDocumentTitle(getString('appsOnboarding'));

  const breadcrumbs = [
    {
      label: getString('appsOnboarding'),
      url: paths.toAppsOnboarding()
    }
  ];

  const getMethodLabel = (method: OnboardingMethod): string => {
    switch (method) {
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
    if (selectedMethod && uploadedFile) {
      const environmentInfo = selectedMethod === OnboardingMethod.HELM_CHART && selectedEnvironment 
        ? ` in environment: ${environments.find(env => env.environmentID === selectedEnvironment)?.name || selectedEnvironment}`
        : '';
      showSuccess(`You have selected: ${getMethodLabel(selectedMethod)} with file: ${uploadedFile.name}${environmentInfo}`);
    }
  };

  const handleUploadClick = (): void => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
    const file = event.target.files?.[0];
    if (file && selectedMethod) {
      setUploadedFile({ name: file.name, method: selectedMethod });
      showSuccess(getString('uploadedSuccessfully'));
    }
    // Reset the input so the same file can be selected again if needed
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const getAcceptedFileTypes = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return '.yaml,.yml,.tgz';
      case OnboardingMethod.CLOUD_MANAGED:
        return '.yaml,.yml,.json';
      default:
        return '*';
    }
  };

  const isOnboardDisabled = (): boolean => {
    if (!selectedMethod) return true;
    
    switch (selectedMethod) {
      case OnboardingMethod.HELM_CHART:
        return !uploadedFile || uploadedFile.method !== selectedMethod || !selectedEnvironment;
      case OnboardingMethod.CLOUD_MANAGED:
        return !cloudManagedUrl.trim() || !uploadedFile || uploadedFile.method !== selectedMethod;
      default:
        return true;
    }
  };

  const getTextboxValue = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return uploadedFile && uploadedFile.method === method ? uploadedFile.name : '';
      case OnboardingMethod.CLOUD_MANAGED:
        return cloudManagedUrl;
      default:
        return '';
    }
  };

  const handleTextboxChange = (method: OnboardingMethod, value: string): void => {
    if (method === OnboardingMethod.CLOUD_MANAGED) {
      setCloudManagedUrl(value);
    }
  };

  const getTextboxPlaceholder = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return getString('uploadedFileName');
      case OnboardingMethod.CLOUD_MANAGED:
        return getString('enterCloudManagedUrl');
      default:
        return '';
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

            <input
              ref={fileInputRef}
              type="file"
              accept={selectedMethod ? getAcceptedFileTypes(selectedMethod) : '*'}
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />

            <div className={css.radioGroup}>
              {radioOptions.map(option => (
                <div key={option.value} className={css.radioRow}>
                  <label
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
                  {selectedMethod === option.value && (
                    <Layout.Vertical spacing="medium" className={css.inputSection}>
                      {/* First row: Upload filename and button */}
                      <Layout.Horizontal spacing="medium" style={{ alignItems: 'center', justifyContent: 'flex-start' }}>
                        <div style={{ flex: 1 }}>
                          <TextInput
                            placeholder={getTextboxPlaceholder(option.value)}
                            value={getTextboxValue(option.value)}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleTextboxChange(option.value, e.target.value)}
                            disabled={option.value === OnboardingMethod.HELM_CHART}
                            className={css.urlTextbox}
                          />
                        </div>
                        <Button
                          variation={ButtonVariation.SECONDARY}
                          text={getString('upload')}
                          icon="upload"
                          onClick={handleUploadClick}
                          className={css.uploadButton}
                        />
                        {uploadedFile && uploadedFile.method === option.value && (
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREEN_700} className={css.uploadedFileName}>
                            ✓
                          </Text>
                        )}
                      </Layout.Horizontal>
                      
                      {/* Second row: Environment dropdown (only for Helm Chart) */}
                      {option.value === OnboardingMethod.HELM_CHART && (
                        <div style={{ alignSelf: 'flex-start', width: '100%', maxWidth: '400px' }}>
                          <Text font={{ variation: FontVariation.FORM_LABEL }} color={Color.GREY_800} style={{ marginBottom: '8px' }}>
                            Select Environment
                          </Text>
                          {environmentsLoading ? (
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                              Loading environments...
                            </Text>
                          ) : environmentsError ? (
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>
                              Error loading environments
                            </Text>
                          ) : environments.length === 0 ? (
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                              No environments found
                            </Text>
                          ) : (
                            <DropDown
                              value={selectedEnvironment}
                              items={environmentOptions}
                              onChange={(selectedOption: SelectOption) => setSelectedEnvironment(selectedOption.value as string)}
                              placeholder="Choose an environment"
                              disabled={environmentsLoading}
                            />
                          )}
                        </div>
                      )}
                    </Layout.Vertical>
                  )}
                </div>
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
                disabled={isOnboardDisabled()}
              />
            </Container>
          </Layout.Vertical>
        )}
      </Container>
    </DefaultLayoutTemplate>
  );
}
