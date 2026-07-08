import React, { useState } from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Card, Container, Layout, Tag, Text } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { ApplicationSpec, CatalogAppInput } from '@api/entities';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import css from './AppDetail.module.scss';

interface AppDetailViewProps {
  app: ApplicationSpec | null | undefined;
  loading: boolean;
}

function SuitabilityList({ items, suitable }: { items: string[]; suitable: boolean }): React.ReactElement {
  return (
    <Layout.Vertical spacing="xsmall">
      {items.map((item, idx) => (
        <Layout.Horizontal key={idx} spacing="xsmall" flex={{ alignItems: 'flex-start' }}>
          <Text color={suitable ? Color.GREEN_600 : Color.RED_600} font={{ variation: FontVariation.BODY }}>
            {suitable ? '✅' : '❌'}
          </Text>
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>{item}</Text>
        </Layout.Horizontal>
      ))}
    </Layout.Vertical>
  );
}

function InputField({ input }: { input: CatalogAppInput }): React.ReactElement {
  return (
    <Layout.Vertical spacing="xsmall" className={css.inputField}>
      <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
        <Text font={{ variation: FontVariation.FORM_LABEL }} color={Color.GREY_700}>
          {input.displayName}
        </Text>
        {input.required && (
          <Text font={{ variation: FontVariation.SMALL }} color={Color.RED_600}>*</Text>
        )}
      </Layout.Horizontal>
      {input.description && (
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>{input.description}</Text>
      )}
      <div className={css.inputPlaceholder}>
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
          {input.default ?? (input.type === 'enum' ? input.values?.[0] : '')}
          {input.unit ? ` ${input.unit}` : ''}
        </Text>
      </div>
    </Layout.Vertical>
  );
}

export default function AppDetailView({ app, loading }: AppDetailViewProps): React.ReactElement {
  const paths = useRouteWithBaseUrl();
  const [showAdvanced, setShowAdvanced] = useState(false);

  useDocumentTitle(app?.displayName ?? 'App Detail');

  const breadcrumbs = [{ label: 'App Catalog', url: paths.toAppsHub() }];

  if (loading) {
    return (
      <DefaultLayoutTemplate title="App Catalog" breadcrumbs={breadcrumbs}>
        <Container padding="xlarge">
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>Loading...</Text>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  if (!app) {
    return (
      <DefaultLayoutTemplate title="App Catalog" breadcrumbs={breadcrumbs}>
        <Container padding="xlarge">
          <Layout.Vertical flex={{ justifyContent: 'center', alignItems: 'center' }} height={400} spacing="medium">
            <Icon name="nav-settings" size={48} color={Color.GREY_400} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>Application not found</Text>
          </Layout.Vertical>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  const standardInputs = app.inputs.filter(i => !i.advanced);
  const advancedInputs = app.inputs.filter(i => i.advanced);
  const compatibleFaults = app.faultCompatibility.filter(f => f.compatible);
  const incompatibleFaults = app.faultCompatibility.filter(f => !f.compatible);

  return (
    <DefaultLayoutTemplate title={app.displayName} breadcrumbs={breadcrumbs}>
      <Container padding="xlarge" className={css.container}>
        <Layout.Horizontal spacing="xlarge" flex={{ alignItems: 'flex-start' }}>
          <Card className={css.detailCard} elevation={1}>
            <Layout.Vertical spacing="large" padding="xlarge">
              <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center' }}>
                <Icon name="nav-settings" size={36} color={Color.PRIMARY_7} />
                <Layout.Vertical spacing="xsmall">
                  <Text font={{ variation: FontVariation.H3 }} color={Color.GREY_800}>{app.displayName}</Text>
                  <Layout.Horizontal spacing="small">
                    <Tag>{app.tier === 'official' ? 'Official' : 'Community'}</Tag>
                    <Tag>{app.domain}</Tag>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>v{app.version}</Text>
                  </Layout.Horizontal>
                </Layout.Vertical>
              </Layout.Horizontal>

              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                {app.description.long}
              </Text>

              {app.description.suitableFor.length > 0 && (
                <Layout.Vertical spacing="small">
                  <SuitabilityList items={app.description.suitableFor} suitable={true} />
                  {app.description.notSuitableFor.length > 0 && (
                    <SuitabilityList items={app.description.notSuitableFor} suitable={false} />
                  )}
                </Layout.Vertical>
              )}

              <Layout.Vertical spacing="small">
                <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>
                  Microservices ({app.microservices.length})
                </Text>
                <div className={css.tagCloud}>
                  {app.microservices.map(ms => (
                    <Tag key={ms.name}>{ms.name}</Tag>
                  ))}
                </div>
              </Layout.Vertical>

              <Layout.Vertical spacing="small">
                <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>Available Faults</Text>
                <div className={css.tagCloud}>
                  {compatibleFaults.map(f => <Tag key={f.faultName}>{f.faultName}</Tag>)}
                </div>
                {incompatibleFaults.length > 0 && (
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>
                    + {incompatibleFaults.length} incompatible faults
                  </Text>
                )}
              </Layout.Vertical>

              <Layout.Horizontal spacing="xlarge">
                <Layout.Vertical spacing="xsmall">
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Namespace</Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{app.install.namespace.default}</Text>
                </Layout.Vertical>
                <Layout.Vertical spacing="xsmall">
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Install timeout</Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{app.install.timeout}</Text>
                </Layout.Vertical>
              </Layout.Horizontal>

              <Layout.Horizontal spacing="medium">
                <Button variation={ButtonVariation.TERTIARY} text="View Documentation" icon="link" />
                <Button variation={ButtonVariation.PRIMARY} text="Select This App →" />
              </Layout.Horizontal>
            </Layout.Vertical>
          </Card>

          <Card className={css.configCard} elevation={1}>
            <Layout.Vertical spacing="medium" padding="large">
              <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
                Configure: {app.displayName}
              </Text>

              {standardInputs.length > 0 && (
                <Layout.Vertical spacing="medium">
                  {standardInputs.map(input => <InputField key={input.key} input={input} />)}
                </Layout.Vertical>
              )}

              {advancedInputs.length > 0 && (
                <Layout.Vertical spacing="medium">
                  <div className={css.advancedToggle} onClick={() => setShowAdvanced(v => !v)}>
                    <Icon name={showAdvanced ? 'chevron-down' : 'chevron-right'} size={12} />
                    <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_600}>Advanced</Text>
                  </div>
                  {showAdvanced && (
                    <Layout.Vertical spacing="medium" className={css.advancedSection}>
                      {advancedInputs.map(input => <InputField key={input.key} input={input} />)}
                    </Layout.Vertical>
                  )}
                </Layout.Vertical>
              )}
            </Layout.Vertical>
          </Card>
        </Layout.Horizontal>
      </Container>
    </DefaultLayoutTemplate>
  );
}
