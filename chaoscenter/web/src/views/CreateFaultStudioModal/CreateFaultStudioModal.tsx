import React from 'react';
import {
  Button,
  ButtonVariation,
  Container,
  FormInput,
  Layout,
  Text,
  SelectOption,
  useToaster
} from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import { Form, Formik, FormikProps } from 'formik';
import * as Yup from 'yup';
import type { MutationFunction } from '@apollo/client';
import { Color, FontVariation } from '@harnessio/design-system';
import { Dialog } from '@blueprintjs/core';
import { getScope } from '@utils';
import { useStrings } from '@strings';
import type { ChaosHub } from '@api/entities';
import type { CreateFaultStudioRequest, CreateFaultStudioResponse } from '@api/core';
import Loader from '@components/Loader';
import css from './CreateFaultStudioModal.module.scss';

export interface CreateFaultStudioFormData {
  name: string;
  description: string;
  tags: string[];
  sourceHubId: string;
  isActive: boolean;
}

interface CreateFaultStudioModalProps {
  isOpen: boolean;
  onClose: () => void;
  chaosHubs: ChaosHub[];
  chaosHubsLoading: boolean;
  createFaultStudioMutation: MutationFunction<CreateFaultStudioResponse, CreateFaultStudioRequest>;
  createLoading: boolean;
  onSuccess: () => void;
}

export default function CreateFaultStudioModal({
  isOpen,
  onClose,
  chaosHubs,
  chaosHubsLoading,
  createFaultStudioMutation,
  createLoading,
  onSuccess
}: CreateFaultStudioModalProps): React.ReactElement {
  const { getString } = useStrings();
  const scope = getScope();
  const { showSuccess, showError } = useToaster();

  const initialValues: CreateFaultStudioFormData = {
    name: '',
    description: '',
    tags: [],
    sourceHubId: '',
    isActive: true
  };

  const validationSchema = Yup.object().shape({
    name: Yup.string()
      .required('Name is required')
      .min(3, 'Name must be at least 3 characters')
      .max(50, 'Name must be less than 50 characters')
      .matches(/^[a-zA-Z0-9-_\s]+$/, 'Name can only contain letters, numbers, hyphens, underscores and spaces'),
    description: Yup.string().max(500, 'Description must be less than 500 characters'),
    sourceHubId: Yup.string().required('Please select a ChaosHub')
  });

  const chaosHubOptions: SelectOption[] = chaosHubs
    .filter(hub => hub.isAvailable)
    .map(hub => ({
      label: hub.name,
      value: hub.id
    }));

  const handleSubmit = async (values: CreateFaultStudioFormData): Promise<void> => {
    try {
      await createFaultStudioMutation({
        variables: {
          projectID: scope.projectID,
          request: {
            name: values.name.trim(),
            description: values.description.trim() || undefined,
            tags: values.tags.length > 0 ? values.tags : undefined,
            sourceHubId: values.sourceHubId,
            selectedFaults: [], // Empty initially, will add faults later
            isActive: values.isActive
          }
        }
      });

      showSuccess(getString('faultStudioCreatedSuccessfully'));
      onSuccess();
      onClose();
    } catch (error: unknown) {
      if (error instanceof Error) {
        showError(error.message);
      } else {
        showError('Failed to create Fault Studio');
      }
    }
  };

  return (
    <Dialog
      isOpen={isOpen}
      onClose={onClose}
      title=""
      className={css.modal}
      canOutsideClickClose={false}
      canEscapeKeyClose={!createLoading}
    >
      <Layout.Vertical padding="xlarge" className={css.modalContent}>
        {/* Header */}
        <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Layout.Vertical spacing="xsmall">
            <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
              {getString('newFaultStudio')}
            </Text>
            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
              Create a new Fault Studio to configure and manage faults for AI agent testing
            </Text>
          </Layout.Vertical>
          <Button
            icon="cross"
            variation={ButtonVariation.ICON}
            onClick={onClose}
            disabled={createLoading}
          />
        </Layout.Horizontal>

        {/* Form */}
        <Formik<CreateFaultStudioFormData>
          initialValues={initialValues}
          validationSchema={validationSchema}
          onSubmit={handleSubmit}
          validateOnChange={true}
          validateOnBlur={true}
        >
          {(formikProps: FormikProps<CreateFaultStudioFormData>) => (
            <Form>
              <Layout.Vertical spacing="large" padding={{ top: 'large' }}>
                {/* Name Field */}
                <FormInput.Text
                  name="name"
                  label={getString('faultStudioName')}
                  placeholder="Enter a unique name for your Fault Studio"
                  disabled={createLoading}
                />

                {/* Description Field */}
                <FormInput.TextArea
                  name="description"
                  label={getString('faultStudioDescription')}
                  placeholder="Describe the purpose of this Fault Studio (optional)"
                  disabled={createLoading}
                />

                {/* Source ChaosHub Selection */}
                <Container>
                  <Text font={{ variation: FontVariation.FORM_LABEL }} margin={{ bottom: 'xsmall' }}>
                    Source ChaosHub *
                  </Text>
                  <Loader loading={chaosHubsLoading} small>
                    {chaosHubOptions.length > 0 ? (
                      <FormInput.Select
                        name="sourceHubId"
                        items={chaosHubOptions}
                        placeholder="Select a ChaosHub to source faults from"
                        disabled={createLoading}
                      />
                    ) : (
                      <Layout.Horizontal
                        padding="medium"
                        className={css.noHubsMessage}
                        flex={{ alignItems: 'center' }}
                        spacing="small"
                      >
                        <Icon name="warning-sign" color={Color.ORANGE_500} size={16} />
                        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                          No ChaosHubs available. Please add a ChaosHub first.
                        </Text>
                      </Layout.Horizontal>
                    )}
                  </Loader>
                </Container>

                {/* Tags Field */}
                <FormInput.KVTagInput
                  name="tags"
                  label="Tags"
                  isArray={true}
                  disabled={createLoading}
                />

                {/* Active Toggle */}
                <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
                  <FormInput.CheckBox
                    name="isActive"
                    label="Activate immediately"
                    disabled={createLoading}
                  />
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                    (Studio will be active upon creation)
                  </Text>
                </Layout.Horizontal>

                {/* Actions */}
                <Layout.Horizontal spacing="medium" padding={{ top: 'large' }}>
                  <Button
                    variation={ButtonVariation.PRIMARY}
                    text={createLoading ? 'Creating...' : 'Create Fault Studio'}
                    type="submit"
                    disabled={createLoading || !formikProps.isValid || !formikProps.dirty}
                    icon={createLoading ? 'loading' : undefined}
                  />
                  <Button
                    variation={ButtonVariation.TERTIARY}
                    text="Cancel"
                    onClick={onClose}
                    disabled={createLoading}
                  />
                </Layout.Horizontal>
              </Layout.Vertical>
            </Form>
          )}
        </Formik>
      </Layout.Vertical>
    </Dialog>
  );
}
