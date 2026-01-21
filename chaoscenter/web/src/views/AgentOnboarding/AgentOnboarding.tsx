import React, { useState, useRef } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2, TextInput } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
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

interface UploadedFile {
  name: string;
  method: OnboardingMethod;
}

interface Agent {
  id: string;
  name: string;
  method: string;
  status: string;
  createdAt: string;
}

// Mock data for agents table
const mockAgents: Agent[] = [
  { id: '1', name: 'Production Agent', method: 'Helm Chart', status: 'Active', createdAt: '2026-01-15' },
  { id: '2', name: 'Staging Agent', method: 'APIs', status: 'Active', createdAt: '2026-01-10' },
  { id: '3', name: 'Dev Agent', method: 'FaaS', status: 'Inactive', createdAt: '2026-01-05' }
];

export default function AgentOnboardingView(): React.ReactElement {
  const { getString } = useStrings();
  const { showSuccess } = useToaster();
  const location = useLocation();
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const searchParams = new URLSearchParams(location.search);
  const showOptions = searchParams.get('step') === 'select';
  const [selectedMethod, setSelectedMethod] = useState<OnboardingMethod | null>(null);
  const [uploadedFile, setUploadedFile] = useState<UploadedFile | null>(null);
  const [apiUrl, setApiUrl] = useState<string>('');
  const [faasUrl, setFaasUrl] = useState<string>('');
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [agents, setAgents] = useState<Agent[]>(mockAgents);

  useDocumentTitle(getString('agentOnboarding'));

  const breadcrumbs = [
    {
      label: getString('agentOnboarding'),
      url: paths.toAgentOnboarding()
    }
  ];

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
    if (selectedMethod && uploadedFile) {
      showSuccess(`You have selected: ${getMethodLabel(selectedMethod)} with file: ${uploadedFile.name}`);
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
      case OnboardingMethod.APIS:
        return '.yaml,.yml,.json';
      case OnboardingMethod.FAAS:
        return '.yaml,.yml,.json,.zip';
      default:
        return '*';
    }
  };

  const isOnboardDisabled = (): boolean => {
    if (!selectedMethod) return true;
    
    switch (selectedMethod) {
      case OnboardingMethod.HELM_CHART:
        return !uploadedFile || uploadedFile.method !== selectedMethod;
      case OnboardingMethod.APIS:
        return !apiUrl.trim() || !uploadedFile || uploadedFile.method !== selectedMethod;
      case OnboardingMethod.FAAS:
        return !faasUrl.trim() || !uploadedFile || uploadedFile.method !== selectedMethod;
      default:
        return true;
    }
  };

  const getTextboxValue = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return uploadedFile && uploadedFile.method === method ? uploadedFile.name : '';
      case OnboardingMethod.APIS:
        return apiUrl;
      case OnboardingMethod.FAAS:
        return faasUrl;
      default:
        return '';
    }
  };

  const handleTextboxChange = (method: OnboardingMethod, value: string): void => {
    switch (method) {
      case OnboardingMethod.APIS:
        setApiUrl(value);
        break;
      case OnboardingMethod.FAAS:
        setFaasUrl(value);
        break;
      default:
        break;
    }
  };

  const getTextboxPlaceholder = (method: OnboardingMethod): string => {
    switch (method) {
      case OnboardingMethod.HELM_CHART:
        return getString('uploadedFileName');
      case OnboardingMethod.APIS:
        return getString('enterApiUrl');
      case OnboardingMethod.FAAS:
        return getString('enterFaasUrl');
      default:
        return '';
    }
  };

  const handleNewAgent = (): void => {
    history.push({ search: '?step=select' });
  };

  const handleBack = (): void => {
    history.push({ search: '' });
  };

  const handleEditAgent = (agent: Agent): void => {
    showSuccess(`Editing agent: ${agent.name}`);
  };

  const handleDeleteAgent = (agent: Agent): void => {
    setAgents(prev => prev.filter(a => a.id !== agent.id));
    showSuccess(`Deleted agent: ${agent.name}`);
  };

  const columns: Column<Agent>[] = [
    {
      Header: getString('name'),
      accessor: 'name',
      Cell: ({ value }: CellProps<Agent>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_900}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('method'),
      accessor: 'method',
      Cell: ({ value }: CellProps<Agent>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('status'),
      accessor: 'status',
      Cell: ({ value }: CellProps<Agent>) => (
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
      Cell: ({ value }: CellProps<Agent>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('actions'),
      id: 'actions',
      Cell: ({ row }: CellProps<Agent>) => (
        <Layout.Horizontal spacing="medium">
          <Text
            className={css.actionLink}
            color={Color.PRIMARY_7}
            onClick={() => handleEditAgent(row.original)}
          >
            {getString('edit')}
          </Text>
          <Text
            className={css.actionLink}
            color={Color.RED_600}
            onClick={() => handleDeleteAgent(row.original)}
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
      title={getString('agentOnboarding')}
    >
      <Container className={css.container}>
        {!showOptions ? (
          <Layout.Vertical spacing="large">
            <Text
              font={{ variation: FontVariation.H3 }}
              color={Color.GREY_800}
              className={css.heading}
            >
              {getString('agentOnboarding')}
            </Text>
            <Container className={css.newAgentButtonContainer}>
              <Button
                variation={ButtonVariation.PRIMARY}
                text={getString('newAgent')}
                icon="plus"
                onClick={handleNewAgent}
              />
            </Container>

            {agents.length > 0 && (
              <Container className={css.tableContainer}>
                <Text
                  font={{ variation: FontVariation.H5 }}
                  color={Color.GREY_800}
                  className={css.tableHeading}
                >
                  {getString('onboardedAgents')}
                </Text>
                <TableV2<Agent>
                  columns={columns}
                  data={agents}
                  className={css.agentsTable}
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
              {getString('onboardYourAgent')}
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
                    <div className={css.inputSection}>
                      <TextInput
                        placeholder={getTextboxPlaceholder(option.value)}
                        value={getTextboxValue(option.value)}
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleTextboxChange(option.value, e.target.value)}
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
                        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREEN_700} className={css.uploadedFileName}>
                          ✓
                        </Text>
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
