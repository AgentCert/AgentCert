import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2, TextInput } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import { Color, FontVariation } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import { getScope } from '@utils';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
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
  file?: File;
}

enum ValidationStatus {
  IDLE = 'idle',
  VALIDATING = 'validating',
  SUCCESS = 'success',
  FAILED = 'failed'
}

interface Application {
  id: string;
  appId?: string;
  name: string;
  method: string;
  status: string;
  createdAt: string;
  environmentId?: string;
  namespace?: string;
  version?: string;
}

// Removed mock data - apps will be fetched from database

export default function AppsOnboardingView(): React.ReactElement {
  const { getString } = useStrings();
  const { showSuccess, showError } = useToaster();
  const location = useLocation();
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const searchParams = new URLSearchParams(location.search);
  const showOptions = searchParams.get('step') === 'select';
  const [selectedMethod, setSelectedMethod] = useState<OnboardingMethod | null>(null);
  const [selectedEnvironment, setSelectedEnvironment] = useState<string>('');
  const [validationStatus, setValidationStatus] = useState<ValidationStatus>(ValidationStatus.IDLE);
  const scope = getScope();
  const [uploadedFile, setUploadedFile] = useState<UploadedFile | null>(null);
  const [cloudManagedUrl, setCloudManagedUrl] = useState<string>('');
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [applications, setApplications] = useState<Application[]>([]);
  const [, setIsLoadingApps] = useState<boolean>(true);
  const [isOnboarding, setIsOnboarding] = useState<boolean>(false);
  // Store release info from validation for cleanup during onboard
  const [validationReleaseInfo, setValidationReleaseInfo] = useState<{
    releaseName: string;
    namespace: string;
    chartName: string;
  } | null>(null);
  // Store YAML content extracted from the helm chart
  const [yamlContent, setYamlContent] = useState<string>('');

  // Fetch registered apps from database
  const fetchApplications = useCallback(async (): Promise<void> => {
    try {
      setIsLoadingApps(true);
      const response = await fetch(`/api/apps?projectId=${scope.projectID}`);
      const result = await response.json();

      if (response.ok && result.success) {
        // Filter to only show apps with 'REGISTERED' status
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const registeredApps = result.apps?.filter((app: any) => app.status === 'REGISTERED') || [];
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const apps = registeredApps.map((app: any) => ({
          id: app.appId,
          appId: app.appId,
          name: app.name,
          method: app.method === 'HELM_CHART' ? 'Helm Chart' : app.method,
          status: app.status,
          createdAt: new Date(app.auditInfo?.createdAt * 1000).toISOString().split('T')[0],
          environmentId: app.environmentId,
          namespace: app.namespace,
          version: app.version
        }));
        setApplications(apps);
      }
    } catch (_error) {
      // Failed to fetch applications
    } finally {
      setIsLoadingApps(false);
    }
  }, [scope.projectID]);

  // Fetch applications on mount
  useEffect(() => {
    fetchApplications();
  }, [fetchApplications]);

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

  const handleOnboard = async (): Promise<void> => {
    if (!selectedMethod || !uploadedFile) {
      return;
    }

    setIsOnboarding(true);

    try {
      // Extract app name from the file name (remove extension and version)
      let appName = uploadedFile.name;
      appName = appName.replace(/\.(tgz|tar\.gz|yaml|yml|json)$/i, '');
      // Remove version suffix if present (e.g., sock-shop-0.1.0 -> sock-shop)
      const versionMatch = appName.match(/^(.+)-(\d+\.\d+\.\d+)$/);
      if (versionMatch) {
        appName = versionMatch[1];
      }

      const registerRequest = {
        projectId: scope.projectID,
        name: appName,
        version: versionMatch ? versionMatch[2] : '1.0.0',
        description: `Application onboarded via ${getMethodLabel(selectedMethod)}`,
        chartName: uploadedFile.name,
        namespace: validationReleaseInfo?.namespace || 'default',
        environmentId: selectedEnvironment || '',
        method: selectedMethod === OnboardingMethod.HELM_CHART ? 'HELM_CHART' : 'CLOUD_MANAGED',
        // Pass release info for cleanup during onboard
        releaseName: validationReleaseInfo?.releaseName || '',
        releaseNamespace: validationReleaseInfo?.namespace || 'default'
      };

      const response = await fetch('/api/apps/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(registerRequest)
      });

      const result = await response.json();

      if (response.ok && result.success) {
        showSuccess(`Application "${appName}" registered successfully!`);
        // Refresh the applications list
        await fetchApplications();
        // Navigate back to the main list
        history.push({ search: '' });
        // Reset form
        setUploadedFile(null);
        setSelectedMethod(null);
        setValidationStatus(ValidationStatus.IDLE);
        setSelectedEnvironment('');
        setValidationReleaseInfo(null);
      } else {
        showError(result.message || 'Failed to register application');
      }
    } catch (_error) {
      showError('Failed to register application');
    } finally {
      setIsOnboarding(false);
    }
  };

  const handleUploadClick = (): void => {
    fileInputRef.current?.click();
  };

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>): Promise<void> => {
    const file = event.target.files?.[0];
    if (file && selectedMethod) {
      setUploadedFile({ name: file.name, method: selectedMethod, file: file });
      setValidationStatus(ValidationStatus.IDLE); // Reset validation when new file is uploaded
      setValidationReleaseInfo(null); // Reset release info when new file is uploaded
      setYamlContent(''); // Reset YAML content

      // For Helm charts (.tgz), extract and display values.yaml content
      if (selectedMethod === OnboardingMethod.HELM_CHART && file.name.endsWith('.tgz')) {
        try {
          // Use pako to decompress gzip and read tar
          const arrayBuffer = await file.arrayBuffer();
          const uint8Array = new Uint8Array(arrayBuffer);

          // Import pako dynamically for decompression
          const pako = await import('pako');
          const decompressed = pako.ungzip(uint8Array);

          // Parse tar file to find values.yaml
          let yamlFound = '';
          let offset = 0;
          while (offset < decompressed.length) {
            // Read tar header (512 bytes)
            const header = decompressed.slice(offset, offset + 512);
            if (header[0] === 0) break; // End of archive

            // Extract filename from header (first 100 bytes)
            const nameBytes = header.slice(0, 100);
            const name = new TextDecoder().decode(nameBytes).replace(/\0/g, '').trim();

            // Extract file size from header (bytes 124-135, octal)
            const sizeBytes = header.slice(124, 136);
            const sizeStr = new TextDecoder().decode(sizeBytes).replace(/\0/g, '').trim();
            const size = parseInt(sizeStr, 8) || 0;

            // Check if this is values.yaml
            if (name.endsWith('values.yaml') || name.endsWith('/values.yaml')) {
              const content = decompressed.slice(offset + 512, offset + 512 + size);
              yamlFound = new TextDecoder().decode(content);
              break;
            }

            // Move to next file (512-byte aligned)
            offset += 512 + Math.ceil(size / 512) * 512;
          }

          if (yamlFound) {
            setYamlContent(yamlFound);
          } else {
            setYamlContent('# values.yaml not found in the chart');
          }
        } catch (_err) {
          setYamlContent('# Failed to extract values.yaml from the chart');
        }
      }

      showSuccess(getString('uploadedSuccessfully'));
    }
    // Reset the input so the same file can be selected again if needed
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  // Validate Helm chart by deploying to local Kubernetes cluster
  const handleValidate = async (): Promise<void> => {
    if (!uploadedFile?.file || selectedMethod !== OnboardingMethod.HELM_CHART) {
      return;
    }

    setValidationStatus(ValidationStatus.VALIDATING);

    try {
      // Create FormData to send the helm package to the backend
      const formData = new FormData();
      formData.append('helmPackage', uploadedFile.file);
      formData.append('environmentId', selectedEnvironment);

      // Call backend API to validate helm chart
      const response = await fetch('/api/validate-helm', {
        method: 'POST',
        body: formData
      });

      const result = await response.json();

      if (response.ok && result.success) {
        setValidationStatus(ValidationStatus.SUCCESS);
        // Store release info for cleanup during onboard
        if (result.releaseName) {
          setValidationReleaseInfo({
            releaseName: result.releaseName,
            namespace: result.namespace || 'default',
            chartName: result.chartName || ''
          });
        }
        showSuccess(getString('validatedSuccessfully'));
      } else {
        setValidationStatus(ValidationStatus.FAILED);
        setValidationReleaseInfo(null);
        showError(getString('validationFailed'));
      }
    } catch (error) {
      setValidationStatus(ValidationStatus.FAILED);
      setValidationReleaseInfo(null);
      showError(getString('validationFailed'));
    }
  };

  const isValidateDisabled = (): boolean => {
    if (selectedMethod !== OnboardingMethod.HELM_CHART) return true;
    if (!uploadedFile || uploadedFile.method !== selectedMethod) return true;
    if (validationStatus === ValidationStatus.VALIDATING) return true;
    return false;
  };

  const getAcceptedFileTypes = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return '.tgz';
      case OnboardingMethod.CLOUD_MANAGED:
        return '.yaml,.yml,.json';
      default:
        return '*';
    }
  };

  const isOnboardDisabled = (): boolean => {
    if (isOnboarding) return true;
    if (!selectedMethod) return true;

    switch (selectedMethod) {
      case OnboardingMethod.HELM_CHART:
        // For Helm Chart, require successful validation before onboarding
        return !uploadedFile || uploadedFile.method !== selectedMethod || validationStatus !== ValidationStatus.SUCCESS;
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
        <Text font={{ variation: FontVariation.BODY }} color={value === 'Active' ? Color.GREEN_700 : Color.GREY_500}>
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
          <Text className={css.actionLink} color={Color.PRIMARY_7} onClick={() => handleEditApplication(row.original)}>
            {getString('edit')}
          </Text>
          <Text className={css.actionLink} color={Color.RED_600} onClick={() => handleDeleteApplication(row.original)}>
            {getString('delete')}
          </Text>
        </Layout.Horizontal>
      )
    }
  ];

  return (
    <DefaultLayoutTemplate breadcrumbs={breadcrumbs} title={getString('appsOnboarding')}>
      <Container className={css.container}>
        {!showOptions ? (
          <Layout.Vertical spacing="large">
            <Text font={{ variation: FontVariation.H3 }} color={Color.GREY_800} className={css.heading}>
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
                <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800} className={css.tableHeading}>
                  {getString('onboardedApplications')}
                </Text>
                <TableV2<Application> columns={columns} data={applications} className={css.appsTable} />
              </Container>
            )}
          </Layout.Vertical>
        ) : (
          <Layout.Vertical spacing="large">
            <Text font={{ variation: FontVariation.H3 }} color={Color.GREY_800} className={css.heading}>
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
                      <Text
                        font={{ variation: FontVariation.SMALL }}
                        color={Color.GREY_500}
                        className={css.radioDescription}
                      >
                        {option.description}
                      </Text>
                    </div>
                  </label>
                  {selectedMethod === option.value && (
                    <div className={css.inputSection}>
                      <div className={css.helmChartInputRow}>
                        <TextInput
                          placeholder={getTextboxPlaceholder(option.value)}
                          value={getTextboxValue(option.value)}
                          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                            handleTextboxChange(option.value, e.target.value)
                          }
                          disabled={option.value === OnboardingMethod.HELM_CHART}
                          className={css.urlTextbox}
                        />
                        <Button
                          variation={ButtonVariation.SECONDARY}
                          text={getString('upload')}
                          icon="upload"
                          onClick={handleUploadClick}
                          className={css.uploadButton}
                        />
                        {uploadedFile && uploadedFile.method === option.value && (
                          <Text
                            font={{ variation: FontVariation.SMALL }}
                            color={Color.GREEN_700}
                            className={css.uploadedFileName}
                          >
                            ✓
                          </Text>
                        )}
                      </div>
                      {option.value === OnboardingMethod.HELM_CHART && yamlContent && (
                        <div className={css.yamlContentContainer}>
                          <Text
                            font={{ variation: FontVariation.BODY }}
                            color={Color.GREY_800}
                            className={css.yamlLabel}
                          >
                            values.yaml
                          </Text>
                          <textarea className={css.yamlTextarea} value={yamlContent} readOnly rows={15} />
                        </div>
                      )}
                    </div>
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
              {selectedMethod === OnboardingMethod.HELM_CHART && (
                <Button
                  variation={ButtonVariation.SECONDARY}
                  text={
                    validationStatus === ValidationStatus.VALIDATING ? getString('validating') : getString('validate')
                  }
                  icon={validationStatus === ValidationStatus.VALIDATING ? undefined : 'tick'}
                  onClick={handleValidate}
                  disabled={isValidateDisabled()}
                  className={cx({
                    [css.validateSuccess]: validationStatus === ValidationStatus.SUCCESS,
                    [css.validateFailed]: validationStatus === ValidationStatus.FAILED
                  })}
                >
                  {validationStatus === ValidationStatus.VALIDATING && (
                    <Icon name="loading" size={16} className={css.loadingIcon} />
                  )}
                </Button>
              )}
              <Button
                variation={ButtonVariation.PRIMARY}
                text={isOnboarding ? getString('onboarding') : getString('onboard')}
                onClick={handleOnboard}
                disabled={isOnboardDisabled()}
              >
                {isOnboarding && <Icon name="loading" size={16} className={css.loadingIcon} />}
              </Button>
            </Container>
          </Layout.Vertical>
        )}
      </Container>
    </DefaultLayoutTemplate>
  );
}
