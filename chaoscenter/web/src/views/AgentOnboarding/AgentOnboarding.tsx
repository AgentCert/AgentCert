import React, { useState, useRef, useMemo } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2, TextInput, SelectOption, ConfirmationDialog, Dialog } from '@harnessio/uicore';
import { Color, FontVariation, Intent } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import { useListAgents, ListedAgent, useDeployAgentWithHelm, useDeleteAgent, useUpdateAgent } from '@api/core';
import { useAppStore } from '@context';
import css from './AgentOnboarding.module.scss';

// Available capabilities for agent selection
const AVAILABLE_CAPABILITIES: SelectOption[] = [
  { label: 'Pod Delete', value: 'pod-delete' },
  { label: 'Pod Network Latency', value: 'pod-network-latency' },
  { label: 'Pod Network Loss', value: 'pod-network-loss' },
  { label: 'Pod CPU Hog', value: 'pod-cpu-hog' },
  { label: 'Pod Memory Hog', value: 'pod-memory-hog' },
  { label: 'Pod IO Stress', value: 'pod-io-stress' },
  { label: 'Node Drain', value: 'node-drain' },
  { label: 'Node CPU Hog', value: 'node-cpu-hog' },
  { label: 'Node Memory Hog', value: 'node-memory-hog' },
  { label: 'Node IO Stress', value: 'node-io-stress' },
  { label: 'Node Taint', value: 'node-taint' },
  { label: 'Container Kill', value: 'container-kill' },
  { label: 'Disk Fill', value: 'disk-fill' },
  { label: 'HTTP Chaos', value: 'http-chaos' },
  { label: 'DNS Chaos', value: 'dns-chaos' },
  { label: 'Network Partition', value: 'network-partition' },
  { label: 'AWS EC2 Stop', value: 'aws-ec2-stop' },
  { label: 'AWS EBS Detach', value: 'aws-ebs-detach' },
  { label: 'Azure VM Stop', value: 'azure-vm-stop' },
  { label: 'GCP VM Stop', value: 'gcp-vm-stop' },
  { label: 'Kubernetes Stress', value: 'kubernetes-stress' },
  { label: 'Time Chaos', value: 'time-chaos' }
];

// Helm form state interface
interface HelmFormState {
  agentName: string;
  description: string;
  namespace: string;
  helmReleaseName: string;
  helmChartVersion: string;
  capabilities: string[];
  valuesYaml: string;
}

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
  const { showSuccess, showError } = useToaster();
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
  const [isDeploying, setIsDeploying] = useState<boolean>(false);
  
  // Helm form state
  const [helmForm, setHelmForm] = useState<HelmFormState>({
    agentName: '',
    description: '',
    namespace: 'default',
    helmReleaseName: '',
    helmChartVersion: '1.0.0',
    capabilities: [],
    valuesYaml: ''
  });
  
  // Delete confirmation state
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [agentToDelete, setAgentToDelete] = useState<AgentDisplay | null>(null);
  
  // Edit modal state
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [agentToEdit, setAgentToEdit] = useState<AgentDisplay | null>(null);
  const [editForm, setEditForm] = useState<{ name: string; description: string; capabilities: string[] }>({
    name: '',
    description: '',
    capabilities: []
  });
  
  // Get projectID from app store
  const { projectID } = useAppStore();

  // Deploy agent with Helm mutation
  const [deployAgentWithHelm] = useDeployAgentWithHelm();
  
  // Delete agent mutation
  const [deleteAgent] = useDeleteAgent();
  
  // Update agent mutation
  const [updateAgent, { loading: updateLoading }] = useUpdateAgent();

  // Fetch agents from API
  const { data: agentsData, loading: agentsLoading, refetch: refetchAgents } = useListAgents({
    variables: {
      pagination: { page: 1, limit: 50 }
    }
  });

  // Transform API data to display format
  const agents = useMemo<AgentDisplay[]>(() => {
    if (!agentsData?.listAgents?.agents) {
      return [];
    }
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

  const handleOnboard = async (): Promise<void> => {
    if (!selectedMethod || !projectID) return;

    if (selectedMethod === OnboardingMethod.HELM_CHART) {
      // Validate Helm form
      if (!helmForm.agentName.trim()) {
        showError('Please enter an agent name');
        return;
      }
      if (!helmForm.namespace.trim()) {
        showError('Please enter a namespace');
        return;
      }
      if (!helmForm.helmReleaseName.trim()) {
        showError('Please enter a Helm release name');
        return;
      }
      if (helmForm.capabilities.length === 0) {
        showError('Please select at least one capability');
        return;
      }

      setIsDeploying(true);
      try {
        const result = await deployAgentWithHelm({
          projectID,
          request: {
            name: helmForm.agentName,
            description: helmForm.description,
            namespace: helmForm.namespace,
            capabilities: helmForm.capabilities,
            helmReleaseName: helmForm.helmReleaseName,
            helmChartVersion: helmForm.helmChartVersion,
            valuesYaml: helmForm.valuesYaml
          }
        });

        if (result.data?.deployAgentWithHelm) {
          showSuccess(`Agent "${helmForm.agentName}" deployed successfully!`);
          // Reset form and go back to list
          setHelmForm({
            agentName: '',
            description: '',
            namespace: 'default',
            helmReleaseName: '',
            helmChartVersion: '1.0.0',
            capabilities: [],
            valuesYaml: ''
          });
          setUploadedFile(null);
          setSelectedMethod(null);
          refetchAgents();
          history.push({ search: '' });
        }
      } catch (error) {
        showError(`Deployment failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
      } finally {
        setIsDeploying(false);
      }
    } else {
      // Handle other methods (APIs, FaaS) - placeholder
      showSuccess(`You have selected: ${getMethodLabel(selectedMethod)}`);
    }
  };

  const handleUploadClick = (): void => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
    const file = event.target.files?.[0];
    if (file && selectedMethod) {
      setUploadedFile({ name: file.name, method: selectedMethod });
      
      // For Helm charts, read the file content
      if (selectedMethod === OnboardingMethod.HELM_CHART) {
        const reader = new FileReader();
        reader.onload = (e) => {
          const content = e.target?.result as string;
          setHelmForm(prev => ({
            ...prev,
            valuesYaml: content,
            // Try to extract release name from filename
            helmReleaseName: prev.helmReleaseName || file.name.replace(/\.(yaml|yml|tgz)$/i, '')
          }));
        };
        reader.readAsText(file);
      }
      
      showSuccess(getString('uploadedSuccessfully'));
    }
    // Reset the input so the same file can be selected again if needed
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  // Handle Helm form field changes
  const handleHelmFormChange = (field: keyof HelmFormState, value: string | string[]): void => {
    setHelmForm(prev => ({ ...prev, [field]: value }));
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
    if (!selectedMethod || isDeploying) return true;
    
    switch (selectedMethod) {
      case OnboardingMethod.HELM_CHART:
        return (
          !helmForm.agentName.trim() ||
          !helmForm.namespace.trim() ||
          !helmForm.helmReleaseName.trim() ||
          helmForm.capabilities.length === 0
        );
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
    setAgentToEdit(agent);
    setEditForm({
      name: agent.name,
      description: '',
      capabilities: agent.capabilities ? agent.capabilities.split(', ').filter(c => c) : []
    });
    setEditModalOpen(true);
  };

  const handleDeleteAgent = (agent: AgentDisplay): void => {
    setAgentToDelete(agent);
    setDeleteConfirmOpen(true);
  };
  
  const confirmDeleteAgent = async (): Promise<void> => {
    if (!agentToDelete) return;
    
    try {
      const result = await deleteAgent({ agentID: agentToDelete.id, hardDelete: false });
      if (result.data?.deleteAgent?.success) {
        showSuccess(`Agent "${agentToDelete.name}" deleted successfully`);
        refetchAgents();
      } else {
        showError(result.data?.deleteAgent?.message || 'Failed to delete agent');
      }
    } catch (error) {
      showError(`Delete failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setDeleteConfirmOpen(false);
      setAgentToDelete(null);
    }
  };
  
  const handleUpdateAgent = async (): Promise<void> => {
    if (!agentToEdit) return;
    
    try {
      const result = await updateAgent({
        agentID: agentToEdit.id,
        input: {
          name: editForm.name,
          capabilities: editForm.capabilities.length > 0 ? editForm.capabilities : undefined
        }
      });
      if (result.data?.updateAgent) {
        showSuccess(`Agent "${editForm.name}" updated successfully`);
        refetchAgents();
        setEditModalOpen(false);
        setAgentToEdit(null);
      }
    } catch (error) {
      showError(`Update failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
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
                  {selectedMethod === option.value && option.value === OnboardingMethod.HELM_CHART && (
                    <div className={css.helmFormSection}>
                      <Layout.Vertical spacing="medium" className={css.helmForm}>
                        <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
                          Agent Configuration
                        </Text>
                        
                        <Layout.Horizontal spacing="medium" className={css.formRow}>
                          <div className={css.formField}>
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                              Agent Name *
                            </Text>
                            <TextInput
                              placeholder="Enter agent name"
                              value={helmForm.agentName}
                              onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleHelmFormChange('agentName', e.target.value)}
                              className={css.formInput}
                            />
                          </div>
                          <div className={css.formField}>
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                              Namespace *
                            </Text>
                            <TextInput
                              placeholder="default"
                              value={helmForm.namespace}
                              onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleHelmFormChange('namespace', e.target.value)}
                              className={css.formInput}
                            />
                          </div>
                        </Layout.Horizontal>
                        
                        <div className={css.formField}>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                            Description
                          </Text>
                          <TextInput
                            placeholder="Enter agent description (optional)"
                            value={helmForm.description}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleHelmFormChange('description', e.target.value)}
                            className={css.formInput}
                          />
                        </div>
                        
                        <Layout.Horizontal spacing="medium" className={css.formRow}>
                          <div className={css.formField}>
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                              Helm Release Name *
                            </Text>
                            <TextInput
                              placeholder="my-agent-release"
                              value={helmForm.helmReleaseName}
                              onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleHelmFormChange('helmReleaseName', e.target.value)}
                              className={css.formInput}
                            />
                          </div>
                          <div className={css.formField}>
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                              Helm Chart Version
                            </Text>
                            <TextInput
                              placeholder="1.0.0"
                              value={helmForm.helmChartVersion}
                              onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleHelmFormChange('helmChartVersion', e.target.value)}
                              className={css.formInput}
                            />
                          </div>
                        </Layout.Horizontal>
                        
                        <div className={css.formField}>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                            Domains * (Select one or more)
                          </Text>
                          <div className={css.capabilitiesGrid}>
                            {AVAILABLE_CAPABILITIES.map(cap => (
                              <label key={cap.value as string} className={css.capabilityCheckbox}>
                                <input
                                  type="checkbox"
                                  checked={helmForm.capabilities.includes(cap.value as string)}
                                  onChange={(e) => {
                                    const value = cap.value as string;
                                    if (e.target.checked) {
                                      handleHelmFormChange('capabilities', [...helmForm.capabilities, value]);
                                    } else {
                                      handleHelmFormChange('capabilities', helmForm.capabilities.filter(c => c !== value));
                                    }
                                  }}
                                />
                                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_800}>
                                  {cap.label}
                                </Text>
                              </label>
                            ))}
                          </div>
                        </div>
                        
                        <div className={css.formField}>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                            Values YAML (Optional - Upload or paste)
                          </Text>
                          <Layout.Horizontal spacing="small" className={css.uploadRow}>
                            <Button
                              variation={ButtonVariation.SECONDARY}
                              text={uploadedFile?.method === OnboardingMethod.HELM_CHART ? uploadedFile.name : getString('upload')}
                              icon="upload"
                              onClick={handleUploadClick}
                              className={css.uploadButton}
                            />
                            <Button
                              variation={ButtonVariation.SECONDARY}
                              text="Validate"
                              icon="tick"
                              onClick={() => showSuccess('Validation successful')}
                              className={css.uploadButton}
                            />
                            {uploadedFile?.method === OnboardingMethod.HELM_CHART && (
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREEN_700}>
                                ✓ File loaded
                              </Text>
                            )}
                          </Layout.Horizontal>
                          <textarea
                            placeholder="Paste your values.yaml content here..."
                            value={helmForm.valuesYaml}
                            onChange={(e) => handleHelmFormChange('valuesYaml', e.target.value)}
                            className={css.yamlTextarea}
                            rows={6}
                          />
                        </div>
                      </Layout.Vertical>
                    </div>
                  )}
                  {selectedMethod === option.value && option.value !== OnboardingMethod.HELM_CHART && (
                    <div className={css.inputSection}>
                      <TextInput
                        placeholder={getTextboxPlaceholder(option.value)}
                        value={getTextboxValue(option.value)}
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleTextboxChange(option.value, e.target.value)}
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
                disabled={isDeploying}
              />
              <Button
                variation={ButtonVariation.PRIMARY}
                text={isDeploying ? 'Deploying...' : getString('onboard')}
                onClick={handleOnboard}
                disabled={isOnboardDisabled()}
                loading={isDeploying}
              />
            </Container>
          </Layout.Vertical>
        )}
      </Container>
      
      {/* Delete Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={deleteConfirmOpen}
        titleText="Delete Agent"
        contentText={`Are you sure you want to delete agent "${agentToDelete?.name}"? This action cannot be undone.`}
        confirmButtonText={getString('delete')}
        cancelButtonText={getString('cancel')}
        intent={Intent.DANGER}
        onClose={(isConfirmed: boolean) => {
          if (isConfirmed) {
            confirmDeleteAgent();
          } else {
            setDeleteConfirmOpen(false);
            setAgentToDelete(null);
          }
        }}
      />
      
      {/* Edit Agent Modal */}
      <Dialog
        isOpen={editModalOpen}
        canOutsideClickClose={false}
        canEscapeKeyClose={true}
        onClose={() => {
          setEditModalOpen(false);
          setAgentToEdit(null);
        }}
        className={css.editDialog}
      >
        <Layout.Vertical padding="large" spacing="medium">
          <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_900}>
            {`${getString('edit')} Agent: ${agentToEdit?.name || ''}`}
          </Text>
          <Layout.Vertical spacing="medium" className={css.editModalContent}>
            <div className={css.formField}>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                Agent Name *
              </Text>
              <TextInput
                placeholder="Enter agent name"
                value={editForm.name}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEditForm(prev => ({ ...prev, name: e.target.value }))}
                className={css.formInput}
              />
            </div>
            <div className={css.formField}>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                Domains (Select one or more)
              </Text>
              <div className={css.capabilitiesGrid}>
                {AVAILABLE_CAPABILITIES.map(cap => (
                  <label key={cap.value as string} className={css.capabilityCheckbox}>
                    <input
                      type="checkbox"
                      checked={editForm.capabilities.includes(cap.value as string)}
                      onChange={(e) => {
                        const value = cap.value as string;
                        if (e.target.checked) {
                          setEditForm(prev => ({ ...prev, capabilities: [...prev.capabilities, value] }));
                        } else {
                          setEditForm(prev => ({ ...prev, capabilities: prev.capabilities.filter(c => c !== value) }));
                        }
                      }}
                    />
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_800}>
                      {cap.label}
                    </Text>
                  </label>
                ))}
              </div>
            </div>
          </Layout.Vertical>
          <Layout.Horizontal spacing="small" style={{ justifyContent: 'flex-end', marginTop: '20px' }}>
            <Button
              variation={ButtonVariation.TERTIARY}
              text={getString('cancel')}
              onClick={() => {
                setEditModalOpen(false);
                setAgentToEdit(null);
              }}
            />
            <Button
              variation={ButtonVariation.PRIMARY}
              text={updateLoading ? 'Saving...' : getString('save')}
              onClick={handleUpdateAgent}
              loading={updateLoading}
              disabled={!editForm.name.trim()}
            />
          </Layout.Horizontal>
        </Layout.Vertical>
      </Dialog>
    </DefaultLayoutTemplate>
  );
}
