import React, { useMemo } from 'react';
import { Dialog, Layout, TableV2, Text, useToggleOpen } from '@harnessio/uicore';
import { Color } from '@harnessio/design-system';
import type { Column, Row } from 'react-table';
import { useStrings } from '@strings';
import { useDocumentTitle } from '@hooks';
import { PermissionGroup } from '@models';
import DefaultLayout from '@components/DefaultLayout';
import Loader from '@components/Loader';
import RbacButton from '@components/RbacButton';
import CreateModelConfigController from '@controllers/ModelConfiguration/CreateModelConfig';
import { AliasCell, KeyCell, ModelNameCell, ProviderCell, StatusCell } from './ModelConfigurationListColumns';
import ModelMenuCell from './ModelMenuCell';
import type { ModelRow } from './modelConfigUtils';
import css from './ModelConfiguration.module.scss';

interface ModelConfigurationListViewProps {
  rows: ModelRow[] | undefined;
  loading: boolean;
  refetchModels: () => void;
}

// LLM Models: the project's models and the platform's, as Studio offers them.
export default function ModelConfigurationListView({
  rows,
  loading,
  refetchModels
}: ModelConfigurationListViewProps): React.ReactElement {
  const { getString } = useStrings();
  const { isOpen, open, close } = useToggleOpen();
  useDocumentTitle(getString('llmModels'));

  const columns: Column<ModelRow>[] = useMemo(
    () => [
      { Header: getString('modelAlias').toUpperCase(), id: 'alias', width: '24%', Cell: AliasCell },
      { Header: getString('modelProvider').toUpperCase(), id: 'provider', width: '14%', Cell: ProviderCell },
      { Header: getString('modelName').toUpperCase(), id: 'model', width: '28%', Cell: ModelNameCell },
      { Header: getString('status').toUpperCase(), id: 'status', width: '16%', Cell: StatusCell },
      { Header: getString('key').toUpperCase(), id: 'key', width: '12%', Cell: KeyCell },
      {
        Header: '',
        id: 'threeDotMenu',
        width: '6%',
        Cell: ({ row }: { row: Row<ModelRow> }) => <ModelMenuCell row={row} refetchModels={refetchModels} />,
        disableSortBy: true
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  );

  return (
    <DefaultLayout
      title={getString('llmModels')}
      breadcrumbs={[]}
      subHeader={
        <Layout.Horizontal flex={{ distribution: 'space-between', alignItems: 'center' }} width="100%">
          <Text color={Color.GREY_600} className={css.description}>
            {getString('llmModelsDescription')}
          </Text>
          <RbacButton
            intent="primary"
            data-testid="add-model"
            icon="plus"
            iconProps={{ size: 10 }}
            text={getString('addModel')}
            permission={PermissionGroup.OWNER}
            onClick={open}
          />
        </Layout.Horizontal>
      }
    >
      {isOpen && (
        <Dialog
          isOpen
          canOutsideClickClose={false}
          canEscapeKeyClose={false}
          onClose={close}
          isCloseButtonShown
          className={css.dialog}
        >
          <CreateModelConfigController onClose={close} refetchModels={refetchModels} />
        </Dialog>
      )}
      <Loader
        loading={loading && rows === undefined}
        noData={{
          when: () => rows !== undefined && rows.length === 0,
          messageTitle: getString('noModelsTitle'),
          message: getString('noModelsMessage')
        }}
      >
        <TableV2<ModelRow> columns={columns} data={rows ?? []} />
      </Loader>
    </DefaultLayout>
  );
}
