import { Color, FontVariation } from '@harnessio/design-system';
import { Card, Container, Layout, Text } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import React from 'react';
import { useParams } from 'react-router-dom';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { AppHubCategory, AppHubEntry } from '@api/entities';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import css from './AppDetail.module.scss';

interface AppDetailViewProps {
  categories?: AppHubCategory[];
  loading: boolean;
}

export default function AppDetailView({ categories, loading }: AppDetailViewProps): React.ReactElement {
  const { getString } = useStrings();
  const paths = useRouteWithBaseUrl();
  const { appName } = useParams<{ appName: string }>();

  const app: AppHubEntry | undefined = React.useMemo(() => {
    if (!categories) return undefined;
    for (const cat of categories) {
      const found = cat.applications.find(a => a.name === appName);
      if (found) return found;
    }
    return undefined;
  }, [categories, appName]);

  useDocumentTitle(app?.displayName ?? getString('appsHub'));

  const breadcrumbs = [
    { label: getString('appsHub'), url: paths.toAppsHub() }
  ];

  if (loading) {
    return (
      <DefaultLayoutTemplate title={getString('appsHub')} breadcrumbs={breadcrumbs}>
        <Container padding="xlarge">
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>Loading...</Text>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  if (!app) {
    return (
      <DefaultLayoutTemplate title={getString('appsHub')} breadcrumbs={breadcrumbs}>
        <Container padding="xlarge">
          <Layout.Vertical flex={{ justifyContent: 'center', alignItems: 'center' }} height={400} spacing="medium">
            <Icon name="nav-settings" size={48} color={Color.GREY_400} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
              Application not found
            </Text>
          </Layout.Vertical>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  return (
    <DefaultLayoutTemplate title={app.displayName} breadcrumbs={breadcrumbs}>
      <Container padding="xlarge" className={css.container}>
        <Card className={css.detailCard} elevation={1}>
          <Layout.Vertical spacing="large" padding="xlarge">
            {/* Header */}
            <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center' }}>
              <Icon name="nav-settings" size={36} color={Color.PRIMARY_7} />
              <Layout.Vertical spacing="xsmall">
                <Text font={{ variation: FontVariation.H3 }} color={Color.GREY_800}>
                  {app.displayName}
                </Text>
                <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_500}>
                  v{app.version}
                </Text>
              </Layout.Vertical>
            </Layout.Horizontal>

            {/* Description */}
            <Layout.Vertical spacing="xsmall" className={css.section}>
              <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>Description</Text>
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                {app.description}
              </Text>
            </Layout.Vertical>

            {/* Details */}
            <Layout.Vertical spacing="small" className={css.section}>
              <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>Details</Text>
              <Layout.Horizontal spacing="xlarge">
                <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Name</Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{app.name}</Text>
                </Layout.Vertical>
                <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Version</Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>v{app.version}</Text>
                </Layout.Vertical>
                {app.namespace && (
                  <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Namespace</Text>
                    <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{app.namespace}</Text>
                  </Layout.Vertical>
                )}
                {app.helmReleaseName && (
                  <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Helm Release</Text>
                    <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{app.helmReleaseName}</Text>
                  </Layout.Vertical>
                )}
              </Layout.Horizontal>
            </Layout.Vertical>

            {/* Microservices */}
            {app.microservices && app.microservices.length > 0 && (
              <Layout.Vertical spacing="small">
                <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>
                  Microservices ({app.microservices.length})
                </Text>
                <Layout.Vertical spacing="none" className={css.microserviceList}>
                  <Layout.Horizontal
                    flex={{ justifyContent: 'space-between', alignItems: 'center' }}
                    className={css.microserviceHeader}
                  >
                    <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_600}>Service</Text>
                    <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_600}>Replicas</Text>
                  </Layout.Horizontal>
                  {app.microservices.map(svc => (
                    <Layout.Horizontal
                      key={svc.name}
                      spacing="small"
                      flex={{ justifyContent: 'space-between', alignItems: 'center' }}
                      className={css.microserviceRow}
                    >
                      <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
                        <svg width="8" height="8" viewBox="0 0 8 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                          <circle cx="4" cy="4" r="4" fill={svc.isRunning ? '#0AB000' : '#CF2318'} />
                        </svg>
                        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                          {svc.name}
                        </Text>
                      </Layout.Horizontal>
                      <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                        {svc.readyReplicas}/{svc.desiredReplicas}
                      </Text>
                    </Layout.Horizontal>
                  ))}
                </Layout.Vertical>
              </Layout.Vertical>
            )}
          </Layout.Vertical>
        </Card>
      </Container>
    </DefaultLayoutTemplate>
  );
}
