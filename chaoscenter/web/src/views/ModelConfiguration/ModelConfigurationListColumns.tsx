import React from 'react';
import cx from 'classnames';
import { Layout, Text } from '@harnessio/uicore';
import { Color } from '@harnessio/design-system';
import type { IconName } from '@harnessio/icons';
import type { Row } from 'react-table';
import { ModelSyncStatus } from '@api/entities';
import { useStrings } from '@strings';
import type { StringKeys } from '@strings';
import { ModelRow, PLATFORM_STATUS } from './modelConfigUtils';
import css from './ModelConfiguration.module.scss';

interface ModelCellProps {
  row: Row<ModelRow>;
}

export const AliasCell = ({ row: { original: data } }: ModelCellProps): React.ReactElement => {
  const { getString } = useStrings();
  return (
    <Layout.Vertical spacing="xsmall">
      <Layout.Horizontal spacing="small" flex={{ alignItems: 'center', justifyContent: 'flex-start' }}>
        <Text color={Color.BLACK} lineClamp={1}>
          {data.alias}
        </Text>
        {data.isDefault && <Text className={css.defaultTag}>{getString('platformDefaultModel')}</Text>}
      </Layout.Horizontal>
      {data.label && (
        <Text color={Color.GREY_500} font={{ size: 'small' }} lineClamp={1}>
          {data.label}
        </Text>
      )}
    </Layout.Vertical>
  );
};

export const ProviderCell = ({ row: { original: data } }: ModelCellProps): React.ReactElement => (
  <Text lineClamp={1}>{data.provider}</Text>
);

export const ModelNameCell = ({ row: { original: data } }: ModelCellProps): React.ReactElement => (
  <Text lineClamp={1} color={Color.GREY_700}>
    {data.model ?? '—'}
  </Text>
);

const statusDisplay: Record<ModelRow['status'], { label: StringKeys; icon: IconName; className: string }> = {
  [ModelSyncStatus.ACTIVE]: { label: 'active', icon: 'tick', className: css.active },
  [ModelSyncStatus.PENDING]: { label: 'modelStatusPending', icon: 'time', className: css.working },
  [ModelSyncStatus.DELETING]: { label: 'modelStatusDeleting', icon: 'time', className: css.working },
  [ModelSyncStatus.NEEDS_KEY]: { label: 'modelStatusNeedsKey', icon: 'warning-sign', className: css.needsKey },
  [PLATFORM_STATUS]: { label: 'platform', icon: 'cog', className: css.platform }
};

export const StatusCell = ({ row: { original: data } }: ModelCellProps): React.ReactElement => {
  const { getString } = useStrings();
  const display = statusDisplay[data.status];
  // The reason a model is not active, or where a platform model comes from.
  const tooltip = data.status === PLATFORM_STATUS ? getString('platformModelHelp') : data.lastError ?? undefined;
  return (
    <Text
      className={cx(css.status, display.className)}
      icon={display.icon}
      iconProps={{ size: 12 }}
      tooltip={tooltip}
      tooltipProps={{ isDark: true }}
    >
      {getString(display.label)}
    </Text>
  );
};

export const KeyCell = ({ row: { original: data } }: ModelCellProps): React.ReactElement => (
  <Text font={{ mono: true, size: 'small' }} color={Color.GREY_700}>
    {data.keyHint ?? '—'}
  </Text>
);
