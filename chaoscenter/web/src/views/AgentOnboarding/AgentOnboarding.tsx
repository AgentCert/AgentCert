import React, { useState, useRef, useMemo } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2, TextInput } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import { useListAgents, ListedAgent } from '@api/core';
import { useAppStore } from '@context';
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

// Display format for agents table
interface AgentDisplay {
  id: string;
  name: string;
  namespace: string;
  capabilities: string;
  status: string;
  createdAt: string;
}

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
  
  // Get projectID from app store
  const { projectID } = useAppStore();

  // Fetch agents from API
  const { data: agentsData, loading: agentsLoading } = useListAgents({
    variables: {
      projectID: projectID || '',
      request: {
        pagination: { page: 0, limit: 50 }
      }
    },
    skip: !projectID
  });

  // Transform API data to display format
  const agents = useMemo<AgentDisplay[]>(() => {
    if (!agentsData?.listAgents?.agents) return [];
    return agentsData.listAgents.agents.map((agent: ListedAgent) => ({
      id: agent.agentID,
      name: agent.name,
      namespace: agent.namespace || 'default',
      capabilities: agent.capabilities?.join(', ') || '',
      status: agent.status || 'Unknown',
      createdAt: agent.auditInfo?.createdAt 
        ? new Date(parseInt(agent.auditInfo.createdAt)).toLocaleDateString()
        : 'N/A'
    }));
  }, [agentsData]);

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

  const handleEditAgent = (agent: AgentDisplay): void => {
    showSuccess(`Editing agent: ${agent.name}`);
  };

  const handleDeleteAgent = (agent: AgentDisplay): void => {
    showSuccess(`Delete agent: ${agent.name} - Feature coming soon`);
  };

  const columns: Column<AgentDisplay>[] = [
    {
      Header: getString('name'),
      accessor: 'name',
      Cell: ({ value }: CellProps<AgentDisplay>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_900}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('namespace'),
      accessor: 'namespace',
      Cell: ({ value }: CellProps<AgentDisplay>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('capabilities'),
      accessor: 'capabilities',
      Cell: ({ value }: CellProps<AgentDisplay>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value || 'None'}
        </Text>
      )
    },
    {
      Header: getString('status'),
      accessor: 'status',
      Cell: ({ value }: CellProps<AgentDisplay>) => (
        <Text 
          font={{ variation: FontVariation.BODY }} 
          color={value === 'REGISTERED' || value === 'Active' ? Color.GREEN_700 : Color.GREY_500}
        >
          {value}
        </Text>
      )
    },
    {
      Header: getString('createdAt'),
      accessor: 'createdAt',
      Cell: ({ value }: CellProps<AgentDisplay>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
          {value}
        </Text>
      )
    },
    {
      Header: getString('actions'),
      id: 'actions',
      Cell: ({ row }: CellProps<AgentDisplay>) => (
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

            {agentsLoading ? (
              <Container className={css.tableContainer}>
                <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
                  Loading agents...
                </Text>
              </Container>
            ) : agents.length > 0 ? (
              <Container className={css.tableContainer}>
                <Text
                  font={{ variation: FontVariation.H5 }}
                  color={Color.GREY_800}
                  className={css.tableHeading}
                >
                  {getString('onboardedAgents')}
                </Text>
                <TableV2<AgentDisplay>
                  columns={columns}
                  data={agents}
                  className={css.agentsTable}
                />
              </Container>
            ) : (
              <Container className={css.tableContainer}>
                <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
                  No agents registered yet. Click &quot;New Agent&quot; to get started.
                </Text>
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
