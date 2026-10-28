import React from 'react';
import { Button, ButtonVariation, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { useStrings } from '@strings';

interface DeleteModelConfigViewProps {
  alias: string;
  deleting: boolean;
  // Why the delete was refused, e.g. the experiments still using the model.
  error?: string;
  onConfirm: () => void;
  onClose: () => void;
}

export default function DeleteModelConfigView({
  alias,
  deleting,
  error,
  onConfirm,
  onClose
}: DeleteModelConfigViewProps): React.ReactElement {
  const { getString } = useStrings();
  return (
    <Layout.Vertical padding="small" spacing="medium">
      <Text font={{ variation: FontVariation.H4 }}>{getString('delete')}</Text>
      <Text font={{ variation: FontVariation.BODY }}>{getString('deleteModelConfirm', { alias })}</Text>
      {error && (
        <Text icon="warning-sign" color={Color.RED_700} data-testid="delete-model-error">
          {error}
        </Text>
      )}
      <Layout.Horizontal spacing="small">
        <Button
          variation={ButtonVariation.PRIMARY}
          intent="danger"
          text={getString('confirm')}
          loading={deleting}
          disabled={deleting}
          onClick={onConfirm}
        />
        <Button variation={ButtonVariation.SECONDARY} text={getString('cancel')} onClick={onClose} />
      </Layout.Horizontal>
    </Layout.Vertical>
  );
}
