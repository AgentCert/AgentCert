import React from 'react';
import {
  Button,
  ButtonVariation,
  Container,
  FormInput,
  Layout,
  Text,
  useToaster
} from '@harnessio/uicore';
import { Form, Formik, FormikProps } from 'formik';
import * as Yup from 'yup';
import type { MutationFunction } from '@apollo/client';
import { Color, FontVariation } from '@harnessio/design-system';
import { Dialog } from '@blueprintjs/core';
import { getScope } from '@utils';
import { useStrings } from '@strings';
import type { FaultStudio } from '@api/entities';
import type { UpdateFaultStudioRequest, UpdateFaultStudioResponse } from '@api/core';
import css from './EditFaultStudioModal.module.scss';

export interface EditFaultStudioFormData {
  name: string;
  description: string;
  tags: string[];
  isActive: boolean;
}

interface EditFaultStudioModalProps {
  isOpen: boolean;
  onClose: () => void;
  faultStudio: FaultStudio;
  updateFaultStudioMutation: MutationFunction<UpdateFaultStudioResponse, UpdateFaultStudioRequest>;
  updateLoading: boolean;
  onSuccess: () => void;
}

export default function EditFaultStudioModal({
  isOpen,
  onClose,
  faultStudio,
  updateFaultStudioMutation,
  updateLoading,
  onSuccess
}: EditFaultStudioModalProps): React.ReactElement {
  const { getString } = useStrings();
  const scope = getScope();
  const { showSuccess, showError } = useToaster();

  const initialValues: EditFaultStudioFormData = {
    name: faultStudio.name || '',
    description: faultStudio.description || '',
    tags: faultStudio.tags || [],
    isActive: faultStudio.isActive ?? true
  };

  const validationSchema = Yup.object().shape({
    name: Yup.string()
      .required('Name is required')
      .min(3, 'Name must be at least 3 characters')
      .max(50, 'Name must be less than 50 characters')
      .matches(/^[a-zA-Z0-9-_\s]+$/, 'Name can only contain letters, numbers, hyphens, underscores and spaces'),
    description: Yup.string().max(500, 'Description must be less than 500 characters')
  });

  const handleSubmit = async (values: EditFaultStudioFormData): Promise<void> => {
    try {
      await updateFaultStudioMutation({
        variables: {
          projectID: scope.projectID,
          studioID: faultStudio.id,
          request: {
            name: values.name.trim(),
            description: values.description.trim() || undefined,
            tags: values.tags.length > 0 ? values.tags : undefined,
            isActive: values.isActive
          }
        }
      });

      showSuccess(getString('faultStudioUpdatedSuccessfully'));
      onSuccess();
      onClose();
    } catch (error: unknown) {
      if (error instanceof Error) {
        showError(error.message);
      } else {
        showError('Failed to update Fault Studio');
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
      canEscapeKeyClose={!updateLoading}
    >
      <Layout.Vertical padding="xlarge" className={css.modalContent}>
        {/* Header */}
        <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Layout.Vertical spacing="xsmall">
            <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
              {getString('editFaultStudio')}
            </Text>
            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
              Update the settings for &quot;{faultStudio.name}&quot;
            </Text>
          </Layout.Vertical>
          <Button
            icon="cross"
            variation={ButtonVariation.ICON}
            onClick={onClose}
            disabled={updateLoading}
          />
        </Layout.Horizontal>

        {/* Form */}
        <Formik<EditFaultStudioFormData>
          initialValues={initialValues}
          validationSchema={validationSchema}
          onSubmit={handleSubmit}
          validateOnChange={true}
          validateOnBlur={true}
          enableReinitialize
        >
          {(formikProps: FormikProps<EditFaultStudioFormData>) => (
            <Form>
              <Layout.Vertical spacing="large" padding={{ top: 'large' }}>
                {/* Name Field */}
                <FormInput.Text
                  name="name"
                  label={getString('faultStudioName')}
                  placeholder="Enter a unique name for your Fault Studio"
                  disabled={updateLoading}
                />

                {/* Description Field */}
                <FormInput.TextArea
                  name="description"
                  label={getString('faultStudioDescription')}
                  placeholder="Describe the purpose of this Fault Studio (optional)"
                  disabled={updateLoading}
                />

                {/* Source ChaosHub (Read-only) */}
                <Container>
                  <Text font={{ variation: FontVariation.FORM_LABEL }} margin={{ bottom: 'xsmall' }}>
                    Source ChaosHub
                  </Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                    {faultStudio.sourceHubName || 'Unknown'}
                  </Text>
                  <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_400} margin={{ top: 'xsmall' }}>
                    Source ChaosHub cannot be changed after creation
                  </Text>
                </Container>

                {/* Tags Field */}
                <FormInput.KVTagInput
                  name="tags"
                  label="Tags"
                  isArray={true}
                  disabled={updateLoading}
                />

                {/* Active Toggle */}
                <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
                  <FormInput.CheckBox
                    name="isActive"
                    label="Active"
                    disabled={updateLoading}
                  />
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                    (Inactive studios will not inject faults)
                  </Text>
                </Layout.Horizontal>

                {/* Actions */}
                <Layout.Horizontal spacing="medium" padding={{ top: 'large' }}>
                  <Button
                    variation={ButtonVariation.PRIMARY}
                    text={updateLoading ? 'Saving...' : 'Save Changes'}
                    type="submit"
                    disabled={updateLoading || !formikProps.isValid || !formikProps.dirty}
                    icon={updateLoading ? 'loading' : undefined}
                  />
                  <Button
                    variation={ButtonVariation.TERTIARY}
                    text="Cancel"
                    onClick={onClose}
                    disabled={updateLoading}
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
