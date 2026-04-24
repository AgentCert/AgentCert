import React, { FormEvent } from 'react';
import { Button, ButtonVariation, Container, Layout, RadioButtonGroup, Text, TextInput } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Form, Formik } from 'formik';
import { useParams } from 'react-router-dom';
import { useSearchParams, useUpdateSearchParams } from '@hooks';
import { useStrings } from '@strings';
import { ExperimentType, InfrastructureType } from '@api/entities';
import experimentYamlService, { KubernetesYamlService } from '@services/experiment';
import type { Experiment } from '@db';
import { CronWorkflow, StudioMode, StudioTabs } from '@models';
import { ExpressionBreakdownInterface, getBreakdownValues, getSelectedTab } from '@utils';
import SchedulePanel from '@components/SchedulePanel';
import css from './StudioSchedule.module.scss';

interface SchedulePanelInterface extends ExpressionBreakdownInterface {
  type: ExperimentType;
  expression: string;
  selectedScheduleTab?: string;
  maxRuns?: number;
  delayHours?: number;
  delayMinutes?: number;
  delaySeconds?: number;
}

interface StudioScheduleViewProps {
  mode: StudioMode;
}

export default function StudioScheduleView({ mode }: StudioScheduleViewProps): React.ReactElement {
  const { getString } = useStrings();
  const [currentExperiment, setCurrentExperiment] = React.useState<Experiment | undefined>();
  const searchParams = useSearchParams();
  const updateSearchParams = useUpdateSearchParams();
  const hasUnsavedChangesInURL = searchParams.get('unsavedChanges') === 'true';
  const { experimentKey } = useParams<{ experimentKey: string }>();
  const experimentHandler = experimentYamlService.getInfrastructureTypeHandler(InfrastructureType.KUBERNETES);

  const setUnsavedChanges = (): void => {
    if (!hasUnsavedChangesInURL) updateSearchParams({ unsavedChanges: 'true' });
  };

  React.useEffect(() => {
    experimentHandler?.getExperiment(experimentKey).then(experiment => {
      setCurrentExperiment(experiment);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [experimentKey]);

  const initialExperimentType = experimentHandler?.getExperimentScheduleType(currentExperiment?.manifest);
  const initialCronExpression =
    (currentExperiment?.manifest as CronWorkflow | undefined)?.spec.schedule ?? '0 0/1 * * *';

  return (
    <Container padding="large" background={Color.PRIMARY_BG} className={css.mainContainer}>
      {currentExperiment && (
        <Formik<SchedulePanelInterface>
          initialValues={{
            expression: initialCronExpression,
            ...getBreakdownValues(initialCronExpression),
            selectedScheduleTab: getSelectedTab(initialCronExpression),
            type: initialExperimentType ?? ExperimentType.NON_CRON,
            maxRuns: 1,
            delayHours: 0,
            delayMinutes: 2,
            delaySeconds: 0
          }}
          onSubmit={values => {
            if (values.type === ExperimentType.NON_CRON && initialExperimentType === ExperimentType.CRON) {
              (experimentHandler as KubernetesYamlService)?.convertToNonCronExperiment(experimentKey).then(() => {
                updateSearchParams({ experimentType: ExperimentType.NON_CRON, unsavedChanges: 'true' });
              });
            } else if (values.type === ExperimentType.CRON && initialExperimentType === ExperimentType.NON_CRON) {
              (experimentHandler as KubernetesYamlService)
                ?.convertToCronExperiment(experimentKey, values.expression)
                .then(() => {
                  updateSearchParams({ experimentType: ExperimentType.CRON, unsavedChanges: 'true' });
                });
            } else if (
              values.type === ExperimentType.CRON &&
              initialExperimentType === ExperimentType.CRON &&
              values.expression !== (currentExperiment?.manifest as CronWorkflow).spec.schedule
            ) {
              (experimentHandler as KubernetesYamlService)
                ?.updateCronExpression(experimentKey, values.expression)
                .then(() => {
                  setUnsavedChanges();
                });
            } else if (values.type === ExperimentType.MULTI_RUN) {
              // Calculate delay in seconds from hours, minutes, seconds
              const delaySeconds = ((values.delayHours ?? 0) * 3600) + ((values.delayMinutes ?? 2) * 60) + (values.delaySeconds ?? 0);
              // Store maxRuns and delay in experiment metadata for multi-run execution
              (experimentHandler as KubernetesYamlService)?.setMultiRunConfig(experimentKey, values.maxRuns ?? 1, delaySeconds).then(() => {
                updateSearchParams({ experimentType: ExperimentType.MULTI_RUN, maxRuns: String(values.maxRuns ?? 1), unsavedChanges: 'true' });
              });
            }
          }}
        >
          {formikProps => {
            return (
              <Form className={css.formContainer}>
                <RadioButtonGroup
                  name="type"
                  label={<Text font={{ variation: FontVariation.CARD_TITLE }}>Select Schedule</Text>}
                  inline={false}
                  selectedValue={formikProps.values.type}
                  onChange={(e: FormEvent<HTMLInputElement>) => {
                    formikProps.setFieldValue('type', e.currentTarget.value);
                  }}
                  options={[
                    {
                      label: (
                        <Layout.Vertical spacing={'small'}>
                          <Text font={{ variation: FontVariation.H6 }}>{getString('nonCronSelectOption')}</Text>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                            {getString('nonCronText')}
                          </Text>
                        </Layout.Vertical>
                      ),
                      value: ExperimentType.NON_CRON,
                      disabled: formikProps.initialValues.type === ExperimentType.CRON && mode === StudioMode.EDIT
                    },
                    {
                      label: (
                        <Layout.Vertical spacing={'small'}>
                          <Text font={{ variation: FontVariation.H6 }}>{getString('cronSelectOption')}</Text>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                            {getString('cronText')}
                          </Text>
                          {formikProps.values.type === ExperimentType.CRON && (
                            <SchedulePanel renderFormTitle={false} hideSeconds formikProps={formikProps} />
                          )}
                        </Layout.Vertical>
                      ),
                      value: ExperimentType.CRON
                    },
                    {
                      label: (
                        <Layout.Vertical spacing={'small'}>
                          <Text font={{ variation: FontVariation.H6 }}>{getString('multiRunSelectOption')}</Text>
                          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
                            {getString('multiRunText')}
                          </Text>
                          {formikProps.values.type === ExperimentType.MULTI_RUN && (
                            <Layout.Vertical spacing={'medium'} padding={{ top: 'medium' }}>
                              <Text font={{ variation: FontVariation.FORM_LABEL }}>{getString('multiRunMaxRunsLabel')}</Text>
                              <TextInput
                                name="maxRuns"
                                type="number"
                                min={1}
                                max={100}
                                placeholder={getString('multiRunMaxRunsPlaceholder')}
                                value={String(formikProps.values.maxRuns ?? 1)}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                  const value = parseInt(e.target.value, 10);
                                  formikProps.setFieldValue('maxRuns', isNaN(value) ? 1 : Math.max(1, Math.min(100, value)));
                                }}
                                style={{ width: '300px' }}
                              />
                              <Text font={{ variation: FontVariation.FORM_LABEL }} style={{ marginTop: '16px' }}>{getString('multiRunDelayLabel')}</Text>
                              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} style={{ marginTop: '-4px' }}>{getString('multiRunDelayHint')}</Text>
                              <Layout.Horizontal spacing={'medium'} style={{ width: '300px', marginTop: '16px' }}>
                                <Layout.Vertical spacing={'xsmall'} style={{ flex: 1 }}>
                                  <Text font={{ variation: FontVariation.SMALL }}>{getString('multiRunDelayHours')}</Text>
                                  <TextInput
                                    name="delayHours"
                                    type="number"
                                    min={0}
                                    max={50}
                                    value={String(formikProps.values.delayHours ?? 0)}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                      const value = parseInt(e.target.value, 10);
                                      formikProps.setFieldValue('delayHours', isNaN(value) ? 0 : Math.max(0, Math.min(50, value)));
                                    }}
                                    style={{ width: '80px' }}
                                  />
                                </Layout.Vertical>
                                <Layout.Vertical spacing={'xsmall'} style={{ flex: 1 }}>
                                  <Text font={{ variation: FontVariation.SMALL }}>{getString('multiRunDelayMinutes')}</Text>
                                  <TextInput
                                    name="delayMinutes"
                                    type="number"
                                    min={0}
                                    max={59}
                                    value={String(formikProps.values.delayMinutes ?? 2)}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                      const value = parseInt(e.target.value, 10);
                                      formikProps.setFieldValue('delayMinutes', isNaN(value) ? 0 : Math.max(0, Math.min(59, value)));
                                    }}
                                    style={{ width: '80px' }}
                                  />
                                </Layout.Vertical>
                                <Layout.Vertical spacing={'xsmall'} style={{ flex: 1 }}>
                                  <Text font={{ variation: FontVariation.SMALL }}>{getString('multiRunDelaySeconds')}</Text>
                                  <TextInput
                                    name="delaySeconds"
                                    type="number"
                                    min={0}
                                    max={59}
                                    value={String(formikProps.values.delaySeconds ?? 0)}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                      const value = parseInt(e.target.value, 10);
                                      formikProps.setFieldValue('delaySeconds', isNaN(value) ? 0 : Math.max(0, Math.min(59, value)));
                                    }}
                                    style={{ width: '80px' }}
                                  />
                                </Layout.Vertical>
                              </Layout.Horizontal>
                            </Layout.Vertical>
                          )}
                        </Layout.Vertical>
                      ),
                      value: ExperimentType.MULTI_RUN,
                      disabled: formikProps.initialValues.type === ExperimentType.CRON && mode === StudioMode.EDIT
                    }
                  ]}
                  className={css.radioButton}
                />
                <Layout.Horizontal flex={{ justifyContent: 'flex-start' }} spacing={'medium'}>
                  <Button
                    variation={ButtonVariation.SECONDARY}
                    text={getString('back')}
                    onClick={() => {
                      return updateSearchParams({ tab: StudioTabs.BUILDER });
                    }}
                  />
                  <Button type="submit" intent="primary" text={getString('setSchedule')} />
                </Layout.Horizontal>
              </Form>
            );
          }}
        </Formik>
      )}
    </Container>
  );
}
