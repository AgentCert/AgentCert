import React from 'react';
import { Classes, Menu, Popover, PopoverInteractionKind, Position } from '@blueprintjs/core';
import { Button, ButtonVariation, Dialog, Layout, useToggleOpen } from '@harnessio/uicore';
import type { Row } from 'react-table';
import { ModelSyncStatus } from '@api/entities';
import { useStrings } from '@strings';
import { PermissionGroup } from '@models';
import RbacMenuItem from '@components/RbacMenuItem';
import { killEvent } from '@utils';
import DeleteModelConfigController from '@controllers/ModelConfiguration/DeleteModelConfig';
import RotateModelKeyController from '@controllers/ModelConfiguration/RotateModelKey';
import TestModelConfigController from '@controllers/ModelConfiguration/TestModelConfig';
import type { ModelRow } from './modelConfigUtils';
import css from './ModelConfiguration.module.scss';

interface ModelMenuCellProps {
  row: Row<ModelRow>;
  refetchModels: () => void;
}

// Test, Rotate key and Delete for a project model. Platform models have none:
// they are configured in .env.
export default function ModelMenuCell({
  row: { original: data },
  refetchModels
}: ModelMenuCellProps): React.ReactElement | null {
  const { getString } = useStrings();
  const { open: openTest, isOpen: isTestOpen, close: closeTest } = useToggleOpen();
  const { open: openRotate, isOpen: isRotateOpen, close: closeRotate } = useToggleOpen();
  const { open: openDelete, isOpen: isDeleteOpen, close: closeDelete } = useToggleOpen();
  const config = data.config;
  if (!config) {
    return null;
  }
  // Registering and deleting are still settling with the gateway.
  const busy = config.syncStatus === ModelSyncStatus.PENDING || config.syncStatus === ModelSyncStatus.DELETING;

  return (
    <Layout.Vertical flex={{ justifyContent: 'center', alignItems: 'flex-end' }} onClick={killEvent}>
      <Popover className={Classes.DARK} position={Position.LEFT} interactionKind={PopoverInteractionKind.HOVER}>
        <Button variation={ButtonVariation.ICON} icon="Options" data-testid={`model-menu-${config.alias}`} />
        <Menu style={{ backgroundColor: 'unset' }}>
          <RbacMenuItem
            icon="play"
            text={getString('modelTest')}
            onClick={openTest}
            disabled={busy}
            permission={PermissionGroup.OWNER}
          />
          <RbacMenuItem
            icon="key"
            text={getString('rotateKey')}
            onClick={openRotate}
            disabled={busy}
            permission={PermissionGroup.OWNER}
          />
          <RbacMenuItem
            icon="trash"
            text={getString('delete')}
            onClick={openDelete}
            permission={PermissionGroup.OWNER}
          />
        </Menu>
      </Popover>
      {isTestOpen && (
        <Dialog
          isOpen
          enforceFocus={false}
          onClose={closeTest}
          className={css.smallDialog}
          title={getString('testModel')}
        >
          <TestModelConfigController modelID={config.modelID} onClose={closeTest} />
        </Dialog>
      )}
      {isRotateOpen && (
        <Dialog isOpen enforceFocus={false} onClose={closeRotate} className={css.smallDialog}>
          <RotateModelKeyController config={config} onClose={closeRotate} refetchModels={refetchModels} />
        </Dialog>
      )}
      {isDeleteOpen && (
        <Dialog isOpen enforceFocus={false} onClose={closeDelete} className={css.smallDialog}>
          <DeleteModelConfigController config={config} onClose={closeDelete} refetchModels={refetchModels} />
        </Dialog>
      )}
    </Layout.Vertical>
  );
}
