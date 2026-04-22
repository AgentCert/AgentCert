import type { ApolloQueryResult } from '@apollo/client';
import { Color, FontVariation } from '@harnessio/design-system';
import { Card, Container, Layout, Text } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import React from 'react';
import { useHistory } from 'react-router-dom';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { AppHubCategory, AppHubEntry } from '@api/entities';
import type { ListAppHubCategoriesRequest, ListAppHubCategoriesResponse } from '@api/core';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import Loader from '@components/Loader';
import css from './AppsHub.module.scss';

interface AppsHubViewProps {
  categories?: AppHubCategory[];
  loading: boolean;
  refetch: (
    variables?: Partial<ListAppHubCategoriesRequest> | undefined
  ) => Promise<ApolloQueryResult<ListAppHubCategoriesResponse>>;
}

/* DeploymentStatusBadge hidden as part of UI-changes branch
function DeploymentStatusBadge({ isDeployed, runningServices }: { isDeployed: boolean; runningServices?: string }): React.ReactElement {
  const { getString } = useStrings();
  return (
    <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
      <svg width="8" height="8" viewBox="0 0 8 8" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="4" cy="4" r="4" fill={isDeployed ? '#0AB000' : '#999999'} />
      </svg>
      <Text font={{ variation: FontVariation.SMALL }} color={isDeployed ? Color.GREEN_700 : Color.GREY_500}>
        {isDeployed ? `${runningServices ?? '0/0'} ${getString('runningServices')}` : getString('appNotDeployed')}
      </Text>
    </Layout.Horizontal>
  );
}
*/

/* MicroserviceRow moved to AppDetail page
function MicroserviceRow({ service }: { service: Microservice }): React.ReactElement {
  return (
    <Layout.Horizontal
      spacing="small"
      flex={{ justifyContent: 'space-between', alignItems: 'center' }}
      className={css.microserviceRow}
    >
      <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
        <svg width="6" height="6" viewBox="0 0 6 6" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="3" cy="3" r="3" fill={service.isRunning ? '#0AB000' : '#CF2318'} />
        </svg>
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_700}>
          {service.name}
        </Text>
      </Layout.Horizontal>
      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
        {service.readyReplicas}/{service.desiredReplicas}
      </Text>
    </Layout.Horizontal>
  );
}
*/

function AppCard({ app }: { app: AppHubEntry }): React.ReactElement {
  const history = useHistory();
  const paths = useRouteWithBaseUrl();

  return (
    <Card
      className={css.appCard}
      elevation={1}
      interactive
      onClick={() => history.push(paths.toAppDetail({ appName: app.name }))}
    >
      <Layout.Vertical spacing="medium" padding="medium">
        <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
            <Icon name="nav-settings" size={24} color={Color.PRIMARY_7} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
              {app.displayName}
            </Text>
          </Layout.Horizontal>
        </Layout.Horizontal>
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600} lineClamp={2}>
          {app.description}
        </Text>
        <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_500}>
            v{app.version}
          </Text>
          {app.namespace && (
            <>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>|</Text>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                {app.namespace}
              </Text>
            </>
          )}
        </Layout.Horizontal>
        {app.microservices && app.microservices.length > 0 && (
          <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.PRIMARY_7}>
            {app.microservices.length} microservices
          </Text>
        )}
      </Layout.Vertical>
    </Card>
  );
}

export default function AppsHubView({
  categories,
  loading,
  refetch: _refetch
}: AppsHubViewProps): React.ReactElement {
  const { getString } = useStrings();

  useDocumentTitle(getString('appsHub'));

  return (
    <DefaultLayoutTemplate title={getString('appsHub')} breadcrumbs={[]} subTitle={getString('appsHubDescription')}>
      <Container padding="xlarge">
        <Loader loading={loading}>
          {categories && categories.length > 0 ? (
            <Layout.Vertical spacing="xlarge">
              {categories.map(category => (
                <Layout.Vertical key={category.displayName} spacing="medium">
                  <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
                    {category.displayName}
                  </Text>
                  <Layout.Horizontal spacing="medium" className={css.appGrid}>
                    {category.applications.map(app => (
                      <AppCard key={app.name} app={app} />
                    ))}
                  </Layout.Horizontal>
                </Layout.Vertical>
              ))}
            </Layout.Vertical>
          ) : (
            <Layout.Vertical
              flex={{ justifyContent: 'center', alignItems: 'center' }}
              height={400}
              spacing="medium"
            >
              <Icon name="nav-settings" size={48} color={Color.GREY_400} />
              <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
                No applications available in the hub
              </Text>
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_400}>
                Application charts will appear here once the Apps Hub is synced
              </Text>
            </Layout.Vertical>
          )}
        </Loader>
      </Container>
    </DefaultLayoutTemplate>
  );
}
