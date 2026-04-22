import React, { useState, useRef, useMemo, useEffect } from 'react';
import { Layout, Text, Button, ButtonVariation, Container, useToaster, TableV2, TextInput, DropDown, SelectOption, ConfirmationDialog, Dialog } from '@harnessio/uicore';
import { Color, FontVariation, Intent } from '@harnessio/design-system';
import { useLocation, useHistory } from 'react-router-dom';
import type { Column, CellProps } from 'react-table';
import cx from 'classnames';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { getScope } from '@utils';
import { useStrings } from '@strings';
import {
  useListAgents,
  ListedAgent,
  useDeployAgentWithHelm,
  useDeleteAgent,
  useUpdateAgent,
  listChaosInfraMinimal,
  kubeNamespaceSubscription,
  useValidateHelmDeployment,
  useGetEnvironmentVariables
} from '@api/core';
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
  const envFileInputRef = useRef<HTMLInputElement>(null);
  const [isDeploying, setIsDeploying] = useState<boolean>(false);
  const [selectedInfraID, setSelectedInfraID] = useState<string>('');
  const [uploadedEnvFile, setUploadedEnvFile] = useState<string | null>(null);
  
  // Environment variables state (pre-populated from backend)
  const [envVariables, setEnvVariables] = useState<Record<string, { value: string; isSensitive: boolean }>>({
    'AZURE_OPENAI_ENDPOINT': { value: '', isSensitive: false },
    'AZURE_OPENAI_DEPLOYMENT': { value: '', isSensitive: false },
    'AZURE_OPENAI_API_VERSION': { value: '', isSensitive: false },
    'AZURE_OPENAI_EMBEDDING_DEPLOYMENT': { value: '', isSensitive: false },
    'AZURE_OPENAI_KEY': { value: '', isSensitive: true }
  });

  // Load environment variables from backend (pre-populated from .env in dev)
  const { data: envData } = useGetEnvironmentVariables();

  // Helper function to parse .env file content
  const parseEnvFile = (content: string): Record<string, string> => {
    const parsed: Record<string, string> = {};
    const lines = content.split('\n');
    
    lines.forEach(line => {
      // Skip comments and empty lines
      if (!line.trim() || line.trim().startsWith('#')) {
        return;
      }
      
      const [key, ...valueParts] = line.split('=');
      if (key) {
        const trimmedKey = key.trim();
        const value = valueParts.join('=').trim();
        // Remove quotes if present
        const cleanValue = value.replace(/^['"]|['"]$/g, '');
        parsed[trimmedKey] = cleanValue;
      }
    });
    
    return parsed;
  };

  // Handle .env file upload
  const handleEnvFileChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
    const file = event.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const content = e.target?.result as string;
        const parsed = parseEnvFile(content);
        
        // Merge parsed values into envVariables
        setEnvVariables(prev => {
          const updated = { ...prev };
          Object.keys(updated).forEach(key => {
            if (parsed[key]) {
              updated[key] = {
                ...updated[key],
                value: parsed[key]
              };
            }
          });
          return updated;
        });

        setUploadedEnvFile(file.name);
        showSuccess(`Loaded environment variables from ${file.name}`);
      } catch (error) {
        showError(`Failed to parse .env file: ${error instanceof Error ? error.message : 'Unknown error'}`);
      }
    };
    reader.readAsText(file);
  };

  useEffect(() => {
    const baseEnvVars: Record<string, { value: string; isSensitive: boolean }> = {
      'AZURE_OPENAI_ENDPOINT': { value: '', isSensitive: false },
      'AZURE_OPENAI_DEPLOYMENT': { value: '', isSensitive: false },
      'AZURE_OPENAI_API_VERSION': { value: '', isSensitive: false },
      'AZURE_OPENAI_EMBEDDING_DEPLOYMENT': { value: '', isSensitive: false },
      'AZURE_OPENAI_KEY': { value: '', isSensitive: true }
    };

    if (envData?.getEnvironmentVariables?.length) {
      const merged = { ...baseEnvVars };
      envData.getEnvironmentVariables.forEach(v => {
        merged[v.name] = {
          value: v.value || '',
          isSensitive: Boolean(v.isSensitive)
        };
      });
      setEnvVariables(merged);
      return;
    }

    setEnvVariables(baseEnvVars);
  }, [envData]);
  
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
  
  // Store the actual file blob for .tgz charts
  const [helmChartFile, setHelmChartFile] = useState<File | null>(null);
  const [isValidated, setIsValidated] = useState<boolean>(false);
  const [mergedHelmValues, setMergedHelmValues] = useState<string | null>(null);
  
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
  const scope = getScope();

  // Fetch infra list to resolve infraID for namespace subscription
  const { data: infraData, loading: infraLoading, error: infraError } = listChaosInfraMinimal({
    ...scope,
    pagination: { page: 0, limit: 50 }
  });

  const infrastructureOptions = useMemo<SelectOption[]>(
    () =>
      infraData?.listInfras?.infras?.map(infrastructure => ({
        label: infrastructure.name,
        value: infrastructure.infraID
      })) ?? [],
    [infraData]
  );

  // Fetch available Kubernetes namespaces via subscription
  const { data: namespacesData, loading: namespacesLoading, error: namespacesError } = kubeNamespaceSubscription({
    request: {
      infraID: selectedInfraID || ''
    },
    skip: !selectedInfraID,
    shouldResubscribe: true
  });

  const [namespaceOptions, setNamespaceOptions] = useState<SelectOption[]>([]);
  const isNamespaceLoading = infraLoading || (namespacesLoading && namespaceOptions.length === 0);

  useEffect(() => {
    setNamespaceOptions([]);
    setHelmForm(prev => ({
      ...prev,
      namespace: ''
    }));
  }, [selectedInfraID]);

  // Update namespace options when data is loaded
  useEffect(() => {
    if (namespacesData?.getKubeNamespace?.kubeNamespace) {
      const options = namespacesData.getKubeNamespace.kubeNamespace.map(ns => ({
        label: ns.name,
        value: ns.name
      }));
      setNamespaceOptions(options);

      // Autofill with 'litmus-chaos' if available, otherwise 'default'
      const preferredNamespace = options.some(opt => opt.value === 'litmus-chaos')
        ? 'litmus-chaos'
        : options[0]?.value || 'default';
      
      setHelmForm(prev => ({
        ...prev,
        namespace: preferredNamespace
      }));
    }
  }, [namespacesData]);

  // Deploy agent with Helm mutation
  const [deployAgentWithHelm] = useDeployAgentWithHelm();
  
  // Validate Helm deployment mutation
  const [validateHelmDeployment] = useValidateHelmDeployment();
  
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
        ? new Date(parseInt(agent.auditInfo.createdAt) * 1000).toLocaleDateString()
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

      const azureKey = envVariables['AZURE_OPENAI_KEY']?.value || '';
      if (!azureKey) {
        showError('Please enter AZURE_OPENAI_KEY before deploy');
        return;
      }

      setIsDeploying(true);
      try {
        // Prepare chart data if .tgz file was uploaded
        let chartDataBase64: string | undefined;
        if (helmChartFile) {
          const arrayBuffer = await helmChartFile.arrayBuffer();
          const bytes = new Uint8Array(arrayBuffer);
          chartDataBase64 = btoa(String.fromCharCode(...bytes));
        }

        const effectiveValuesYaml = mergedHelmValues || helmForm.valuesYaml || undefined;

        const result = await deployAgentWithHelm({
          projectID,
          request: {
            name: helmForm.agentName,
            description: helmForm.description,
            namespace: helmForm.namespace,
            capabilities: helmForm.capabilities,
            helmReleaseName: helmForm.helmReleaseName,
            helmChartVersion: helmForm.helmChartVersion,
            valuesYaml: effectiveValuesYaml,
            chartData: chartDataBase64,
            // Pass Azure OpenAI credentials from UI form
            azureOpenAIKey: envVariables['AZURE_OPENAI_KEY']?.value,
            azureOpenAIEndpoint: envVariables['AZURE_OPENAI_ENDPOINT']?.value,
            azureOpenAIDeployment: envVariables['AZURE_OPENAI_DEPLOYMENT']?.value,
            azureOpenAIAPIVersion: envVariables['AZURE_OPENAI_API_VERSION']?.value,
            azureOpenAIEmbeddingDeployment: envVariables['AZURE_OPENAI_EMBEDDING_DEPLOYMENT']?.value
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
          setHelmChartFile(null);
          setIsValidated(false);
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
      setIsValidated(false);
      
      // For Helm charts, handle based on file type
      if (selectedMethod === OnboardingMethod.HELM_CHART) {
        const isTgz = file.name.toLowerCase().endsWith('.tgz') || file.name.toLowerCase().endsWith('.tar.gz');
        
        if (isTgz) {
          // Store the file blob for .tgz charts
          setHelmChartFile(file);
          setHelmForm(prev => ({
            ...prev,
            valuesYaml: '', // Clear values since we're using packaged chart
            helmReleaseName: prev.helmReleaseName || file.name.replace(/\.(tgz|tar\.gz)$/i, '')
          }));
        } else {
          // For YAML files, read as text
          const reader = new FileReader();
          reader.onload = (e) => {
            const content = e.target?.result as string;
            setHelmForm(prev => ({
              ...prev,
              valuesYaml: content,
              helmReleaseName: prev.helmReleaseName || file.name.replace(/\.(yaml|yml)$/i, '')
            }));
          };
          reader.readAsText(file);
          setHelmChartFile(null);
        }
      }
      
      showSuccess('File loaded');
    }
    // Reset the input so the same file can be selected again if needed
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  // Handle environment variable changes
  const handleEnvVarChange = (varName: string, newValue: string): void => {
    setEnvVariables(prev => ({
      ...prev,
      [varName]: { ...prev[varName], value: newValue }
    }));
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
          color={value === 'REGISTERED' || value === 'ACTIVE' ? Color.GREEN_700 : Color.GREY_500}
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

            <input
              ref={envFileInputRef}
              type="file"
              accept=".env"
              style={{ display: 'none' }}
              onChange={handleEnvFileChange}
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
                              Infrastructure *
                            </Text>
                            <DropDown
                              items={infrastructureOptions}
                              value={selectedInfraID}
                              onChange={(selected) => setSelectedInfraID(selected.value as string)}
                              placeholder={infraLoading ? 'Loading infrastructure...' : 'Select infrastructure'}
                              disabled={infraLoading || infrastructureOptions.length === 0}
                            />
                            {!infraLoading && infrastructureOptions.length === 0 && (
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>
                                No infrastructure found. Connect an infrastructure to load namespaces.
                              </Text>
                            )}
                            {infraError && (
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>
                                Unable to load infrastructure.
                              </Text>
                            )}
                          </div>
                        </Layout.Horizontal>

                        <Layout.Horizontal spacing="medium" className={css.formRow}>
                          <div className={css.formField}>
                            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                              Namespace *
                            </Text>
                            <DropDown
                              items={namespaceOptions}
                              value={helmForm.namespace}
                              onChange={(selected) => handleHelmFormChange('namespace', selected.value as string)}
                              placeholder={
                                !selectedInfraID
                                  ? 'Select infrastructure first'
                                  : isNamespaceLoading
                                  ? 'Loading namespaces...'
                                  : 'Select namespace'
                              }
                              disabled={isNamespaceLoading || !selectedInfraID || namespaceOptions.length === 0}
                            />
                            {selectedInfraID && namespacesError && (
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>
                                Unable to load namespaces.
                              </Text>
                            )}
                          </div>
                        </Layout.Horizontal>

                        <div className={css.formField}>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} className={css.fieldLabel}>
                            Values YAML (Optional - Upload or paste)
                          </Text>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} style={{ marginBottom: '8px' }}>
                            Upload a values.yaml file or .tgz Helm chart package, or paste YAML content directly
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
                              text={uploadedEnvFile ? `✓ ${uploadedEnvFile}` : 'Upload .env'}
                              icon="upload"
                              onClick={() => envFileInputRef.current?.click()}
                              className={css.uploadButton}
                            />
                            <Button
                              variation={ButtonVariation.SECONDARY}
                              text="Validate"
                              icon="tick"
                              onClick={async () => {
                                if (!helmForm.helmReleaseName.trim()) {
                                  showError('Please enter a release name');
                                  return;
                                }
                                if (!helmForm.namespace.trim()) {
                                  showError('Please enter a namespace');
                                  return;
                                }

                                const azureKey = envVariables['AZURE_OPENAI_KEY']?.value || '';
                                if (!azureKey) {
                                  showError('Please enter AZURE_OPENAI_KEY before validation');
                                  return;
                                }

                                try {
                                  const result = await validateHelmDeployment({
                                    variables: {
                                      projectID,
                                      request: {
                                        name: helmForm.agentName,
                                        helmReleaseName: helmForm.helmReleaseName,
                                        namespace: helmForm.namespace,
                                        helmChartVersion: helmForm.helmChartVersion,
                                        description: helmForm.description,
                                        version: helmForm.version,
                                        capabilities: helmForm.capabilities.length > 0 ? helmForm.capabilities : [],
                                        chartData: helmChartFile ? null : null,
                                        valuesYaml: helmForm.valuesYaml || null,
                                        kubeconfig: null,
                                        azureOpenAIKey: envVariables['AZURE_OPENAI_KEY']?.value,
                                        azureOpenAIEndpoint: envVariables['AZURE_OPENAI_ENDPOINT']?.value,
                                        azureOpenAIDeployment: envVariables['AZURE_OPENAI_DEPLOYMENT']?.value,
                                        azureOpenAIAPIVersion: envVariables['AZURE_OPENAI_API_VERSION']?.value,
                                        azureOpenAIEmbeddingDeployment: envVariables['AZURE_OPENAI_EMBEDDING_DEPLOYMENT']?.value
                                      },
                                    },
                                  });

                                  console.log('[Validate] Full result:', result);
                                  console.log('[Validate] Merged values:', result.data?.validateHelmDeployment?.mergedValues);

                                  if (result.data?.validateHelmDeployment) {
                                    const validation = result.data.validateHelmDeployment;
                                    
                                    if (validation.valid) {
                                      showSuccess('Helm configuration validated successfully!');
                                      setIsValidated(true);
                                      // Store merged values for display
                                      setMergedHelmValues(validation.mergedValues || '');
                                      (window as any).mergedHelmValues = validation.mergedValues;
                                      console.log('[Validate] Stored merged values to window and state');
                                    } else {
                                      showError(`Validation failed: ${validation.errors?.join(', ') || 'Unknown error'}`);
                                      setIsValidated(false);
                                      setMergedHelmValues(null);
                                    }
                                  }
                                } catch (error) {
                                  console.error('[Validate] Error:', error);
                                  showError(`Validation error: ${error instanceof Error ? error.message : 'Unknown error'}`);
                                  setIsValidated(false);
                                }
                              }}
                              className={css.uploadButton}
                            />
                            {uploadedFile?.method === OnboardingMethod.HELM_CHART && (
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREEN_700}>
                                ✓ File loaded
                              </Text>
                            )}
                            {isValidated && (
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREEN_700}>
                                ✓ Validated
                              </Text>
                            )}
                          </Layout.Horizontal>
                          {helmChartFile ? (
                            <>
                              <div style={{ padding: '12px', background: '#f5f5f5', borderRadius: '4px', marginTop: '8px' }}>
                                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                  <strong>📦 Helm Chart Package:</strong> {helmChartFile.name}
                                </Text>
                                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                                  Size: {(helmChartFile.size / 1024).toFixed(2)} KB
                                </Text>
                                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                                  The chart will be deployed with its default values. To customize, extract values.yaml and upload it instead.
                                </Text>
                              </div>
                              {isValidated && (
                                <div style={{ padding: '12px', background: '#f0f7ff', borderRadius: '4px', marginTop: '12px', border: '1px solid #d0e9ff' }}>
                                  <Text font={{ variation: FontVariation.SMALL }} color={Color.BLUE_900} style={{ marginBottom: '8px' }}>
                                    <strong>✓ Deployment Configuration</strong>
                                  </Text>
                                  <Layout.Vertical spacing="small">
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Release Name:</strong> {helmForm.helmReleaseName}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Namespace:</strong> {helmForm.namespace}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Chart Version:</strong> {helmForm.helmChartVersion}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Agent Name:</strong> {helmForm.agentName}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Capabilities:</strong> {helmForm.capabilities.join(', ') || 'None'}
                                    </Text>
                                    {mergedHelmValues && (
                                      <div style={{ marginTop: '12px', paddingTop: '12px', borderTop: '1px solid #d0e9ff' }}>
                                        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} style={{ marginBottom: '8px' }}>
                                          <strong>Merged Helm Values (with Azure OpenAI):</strong>
                                        </Text>
                                        <div style={{ 
                                          background: '#ffffff', 
                                          border: '1px solid #ccc', 
                                          borderRadius: '3px', 
                                          padding: '8px',
                                          maxHeight: '200px',
                                          overflowY: 'auto',
                                          fontFamily: 'monospace',
                                          fontSize: '11px',
                                          whiteSpace: 'pre-wrap',
                                          wordBreak: 'break-all'
                                        }}>
                                          {mergedHelmValues}
                                        </div>
                                      </div>
                                    )}
                                  </Layout.Vertical>
                                </div>
                              )}
                            </>
                          ) : (
                            <>
                              <textarea
                                placeholder="Paste your values.yaml content here, or leave empty to use default chart values..."
                                value={helmForm.valuesYaml}
                                onChange={(e) => handleHelmFormChange('valuesYaml', e.target.value)}
                                className={css.yamlTextarea}
                                rows={6}
                              />
                              {isValidated && (
                                <div style={{ padding: '12px', background: '#f0f7ff', borderRadius: '4px', marginTop: '12px', border: '1px solid #d0e9ff' }}>
                                  <Text font={{ variation: FontVariation.SMALL }} color={Color.BLUE_900} style={{ marginBottom: '8px' }}>
                                    <strong>✓ Deployment Configuration</strong>
                                  </Text>
                                  <Layout.Vertical spacing="small">
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Release Name:</strong> {helmForm.helmReleaseName}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Namespace:</strong> {helmForm.namespace}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Chart Version:</strong> {helmForm.helmChartVersion}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Agent Name:</strong> {helmForm.agentName}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Capabilities:</strong> {helmForm.capabilities.join(', ') || 'None'}
                                    </Text>
                                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                      <strong>Values Source:</strong> {helmForm.valuesYaml.trim() ? 'Custom values.yaml' : 'Default chart values'}
                                    </Text>
                                    {helmForm.valuesYaml.trim() && (
                                      <>
                                        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700} style={{ marginTop: '8px' }}>
                                          <strong>Custom Values Preview:</strong>
                                        </Text>
                                        <div style={{ background: '#fff', padding: '8px', borderRadius: '3px', maxHeight: '150px', overflowY: 'auto', fontSize: '11px', fontFamily: 'monospace', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                                          {helmForm.valuesYaml.substring(0, 500)}
                                          {helmForm.valuesYaml.length > 500 && '...'}
                                        </div>
                                      </>
                                    )}
                                  </Layout.Vertical>
                                </div>
                              )}
                            </>
                          )}
                        </div>

                        <Layout.Horizontal spacing="medium" className={css.formRow}>
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
                        </Layout.Horizontal>
                        
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
                        
                        {/* Environment Variables Section */}
                        <div style={{ marginTop: '20px', padding: '16px', background: '#f9f9f9', borderRadius: '4px', border: '1px solid #e0e0e0' }}>
                          <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800} style={{ marginBottom: '12px' }}>
                            🔐 Environment Configuration
                          </Text>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600} style={{ marginBottom: '16px' }}>
                            These environment variables will be injected into your deployed agent. Update values as needed or upload a .env file.
                          </Text>
                          
                          <Layout.Vertical spacing="small">
                            {Object.entries(envVariables).map(([varName, varData]) => (
                              <div key={varName} style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '8px' }}>
                                <div style={{ flex: '0 0 200px' }}>
                                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
                                    <strong>{varName}</strong>
                                    {varData.isSensitive && <span style={{ marginLeft: '4px', color: '#e74c3c' }}>🔒</span>}
                                  </Text>
                                </div>
                                <div style={{ flex: '1' }}>
                                  <TextInput
                                    placeholder={varData.isSensitive ? '••••••••' : `Enter ${varName}`}
                                    type={varData.isSensitive && !varData.value.startsWith('***') ? 'password' : 'text'}
                                    value={varData.value}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => handleEnvVarChange(varName, e.target.value)}
                                    style={{ width: '100%' }}
                                  />
                                </div>
                              </div>
                            ))}
                          </Layout.Vertical>
                          
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600} style={{ marginTop: '12px', fontStyle: 'italic' }}>
                            💡 Tip: Upload a .env file to load multiple values at once, or manually edit values above. Click Undo to revert to backend defaults.
                          </Text>
                        </div>
                        
                        <div className={css.formField} style={{ marginTop: '20px' }}>
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
