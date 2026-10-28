import React from 'react';
import { Form, Formik } from 'formik';
import * as Yup from 'yup';
import { Button, ButtonVariation, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import PasswordInput from '@components/PasswordInput';
import { useStrings } from '@strings';

interface RotateModelKeyViewProps {
  alias: string;
  // False for an endpoint that takes no key (Ollama, OpenAI-compatible).
  needsKey: boolean;
  saving: boolean;
  onSubmit: (apiKey: string) => void;
  onClose: () => void;
}

// Replaces a model's API key. Also how a model LiteLLM lost ("Needs key") is
// registered again: ACE kept everything else.
export default function RotateModelKeyView({
  alias,
  needsKey,
  saving,
  onSubmit,
  onClose
}: RotateModelKeyViewProps): React.ReactElement {
  const { getString } = useStrings();
  const schema = Yup.object().shape({
    apiKey: needsKey ? Yup.string().trim().required(getString('required')) : Yup.string()
  });
  return (
    <Formik<{ apiKey: string }>
      initialValues={{ apiKey: '' }}
      validationSchema={schema}
      onSubmit={values => onSubmit(values.apiKey.trim())}
    >
      <Form autoComplete="off">
        <Layout.Vertical padding="small" spacing="medium">
          <Text font={{ variation: FontVariation.H4 }}>{getString('rotateKey')}</Text>
          <Text color={Color.GREY_600}>{getString('rotateKeyDescription', { alias })}</Text>
          {!needsKey && <Text color={Color.GREY_600}>{getString('rotateKeyNoKeyHelp')}</Text>}
          <PasswordInput
            name="apiKey"
            label={needsKey ? getString('modelApiKey') : getString('modelApiKeyOptional')}
            autoComplete="new-password"
          />
          <Layout.Horizontal spacing="small">
            <Button
              variation={ButtonVariation.PRIMARY}
              type="submit"
              text={getString('save')}
              loading={saving}
              disabled={saving}
            />
            <Button variation={ButtonVariation.SECONDARY} text={getString('cancel')} onClick={onClose} />
          </Layout.Horizontal>
        </Layout.Vertical>
      </Form>
    </Formik>
  );
}
