import React, { useState } from 'react';
import {
  Layout,
  Text,
  Button,
  ButtonVariation,
  TableV2,
  useToaster,
  Tag,
  ConfirmationDialog
} from '@harnessio/uicore';
import { Color, FontVariation, Intent } from '@harnessio/design-system';
import type { Column, CellProps } from 'react-table';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import { useListModelConfigs, useDeleteModelConfig } from '@api/core/modelLibrary';
import type { ModelConfig } from '@api/core/modelLibrary';
import { ModelConfigDialog } from '@components/ModelConfigDialog';
import { getScope } from '@utils';

export const ModelLibrary: React.FC = () => {
  const { projectID } = getScope();
  const { showSuccess, showError } = useToaster();
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  const { data, loading, refetch } = useListModelConfigs({
    variables: { projectID },
    fetchPolicy: 'cache-and-network',
  });

  const [deleteModelConfig] = useDeleteModelConfig();

  const configs = data?.listModelConfigs ?? [];

  const handleDelete = async (alias: string) => {
    try {
      await deleteModelConfig({ variables: { projectID, alias } });
      showSuccess(`Deleted ${alias}`);
      refetch();
    } catch (e: unknown) {
      showError((e as Error).message);
    }
    setDeleteTarget(null);
  };

  const statusColor = (status: string) =>
    status === 'active' ? Color.GREEN_600 : status === 'error' ? Color.RED_600 : Color.YELLOW_600;

  const columns: Column<ModelConfig>[] = [
    {
      Header: 'ALIAS',
      accessor: 'alias',
      Cell: ({ value }: CellProps<ModelConfig, string>) => (
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_900}>{value}</Text>
      ),
    },
    {
      Header: 'PROVIDER',
      accessor: 'provider',
      Cell: ({ value }: CellProps<ModelConfig, string>) => (
        <Tag intent="primary">{value.toUpperCase()}</Tag>
      ),
    },
    {
      Header: 'MODEL',
      accessor: 'model',
    },
    {
      Header: 'AGENTS',
      accessor: 'agentsUsing',
      Cell: ({ value }: CellProps<ModelConfig, string[]>) => (
        <Text font={{ variation: FontVariation.SMALL }}>{value.length}</Text>
      ),
    },
    {
      Header: 'STATUS',
      accessor: 'status',
      Cell: ({ value }: CellProps<ModelConfig, string>) => (
        <Text color={statusColor(value)} font={{ variation: FontVariation.SMALL_BOLD }}>
          {value.toUpperCase()}
        </Text>
      ),
    },
    {
      Header: '',
      id: 'actions',
      Cell: ({ row }: CellProps<ModelConfig>) => (
        <Button
          icon="trash"
          variation={ButtonVariation.ICON}
          disabled={row.original.agentsUsing.length > 0}
          tooltip={row.original.agentsUsing.length > 0 ? 'Used by agents' : 'Delete'}
          onClick={() => setDeleteTarget(row.original.alias)}
        />
      ),
    },
  ];

  return (
    <DefaultLayoutTemplate title="Model Library" breadcrumbs={[]}>
      <Layout.Vertical spacing="large" padding="xlarge">
        <Layout.Horizontal style={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Text font={{ variation: FontVariation.H4 }}>LLM Model Configurations</Text>
          <Button
            text="+ Add Model"
            variation={ButtonVariation.PRIMARY}
            onClick={() => setIsAddOpen(true)}
          />
        </Layout.Horizontal>
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
          API keys are stored as K8s Secrets in the litmus namespace. ACE never stores keys in plain text.
        </Text>
        {loading ? (
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
            Loading model configurations...
          </Text>
        ) : (
          <TableV2<ModelConfig>
            data={configs}
            columns={columns}
            minimal={true}
          />
        )}
      </Layout.Vertical>
      <ModelConfigDialog
        isOpen={isAddOpen}
        onClose={() => setIsAddOpen(false)}
        onSaved={() => { setIsAddOpen(false); refetch(); }}
        projectID={projectID}
        mode="add"
      />
      <ConfirmationDialog
        isOpen={!!deleteTarget}
        titleText={`Delete "${deleteTarget}"?`}
        contentText="This will remove the model config, K8s Secret, and LiteLLM entry. This cannot be undone."
        confirmButtonText="Delete"
        cancelButtonText="Cancel"
        intent={Intent.DANGER}
        onClose={(isConfirmed) => {
          if (isConfirmed && deleteTarget) {
            handleDelete(deleteTarget);
          } else {
            setDeleteTarget(null);
          }
        }}
      />
    </DefaultLayoutTemplate>
  );
};

export default ModelLibrary;
