import React, { useState, useMemo } from 'react';
import { Color, FontVariation } from '@harnessio/design-system';
import { Button, ButtonVariation, Card, Container, Layout, Text, TextInput } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import { useHistory } from 'react-router-dom';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { ApplicationSpec } from '@api/entities';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import Loader from '@components/Loader';
import css from './AppsHub.module.scss';

const DOMAINS = [
  { id: 'all', displayName: 'All Domains' },
  { id: 'cloud-native', displayName: 'Cloud Native' },
  { id: 'service-mesh', displayName: 'Service Mesh' },
  { id: 'telecom', displayName: 'Telecom' },
  { id: 'health-it', displayName: 'Health IT' },
  { id: 'itops', displayName: 'IT Operations' },
  { id: 'finops', displayName: 'FinOps / Financial' },
];

function TierBadge({ tier }: { tier: string }): React.ReactElement {
  const isOfficial = tier === 'official';
  return (
    <span className={isOfficial ? css.tierBadgeOfficial : css.tierBadgeCommunity}>
      {isOfficial ? 'Official' : 'Community'}
    </span>
  );
}

function DomainBadge({ domain }: { domain: string }): React.ReactElement {
  return <span className={css.domainBadge}>{domain}</span>;
}

function AppCard({ app }: { app: ApplicationSpec }): React.ReactElement {
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const compatibleFaults = app.faultCompatibility.filter(f => f.compatible).length;

  return (
    <Card
      className={css.appCard}
      elevation={1}
      interactive
      onClick={() => history.push(paths.toAppDetail({ appName: app.name }))}
    >
      <Layout.Vertical spacing="medium" padding="medium">
        <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
            <Icon name="nav-settings" size={24} color={Color.PRIMARY_7} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
              {app.displayName}
            </Text>
          </Layout.Horizontal>
          <TierBadge tier={app.tier} />
        </Layout.Horizontal>

        <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
          <DomainBadge domain={app.domain} />
          <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_400}>v{app.version}</Text>
        </Layout.Horizontal>

        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600} lineClamp={2}>
          {app.description.short}
        </Text>

        <Layout.Horizontal spacing="medium">
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            {app.microservices.length} microservices
          </Text>
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            {compatibleFaults} faults
          </Text>
        </Layout.Horizontal>
      </Layout.Vertical>
    </Card>
  );
}

interface AppsHubViewProps {
  apps: ApplicationSpec[];
  loading: boolean;
}

export default function AppsHubView({ apps, loading }: AppsHubViewProps): React.ReactElement {
  const history = useHistory();
  const paths = useRouteWithBaseUrl();
  const [selectedDomain, setSelectedDomain] = useState<string>('all');
  const [searchTerm, setSearchTerm] = useState<string>('');

  useDocumentTitle('App Catalog');

  const filteredApps = useMemo(() => {
    return apps.filter(app => {
      const domainMatch = selectedDomain === 'all' || app.domain === selectedDomain;
      const searchMatch =
        searchTerm === '' ||
        app.displayName.toLowerCase().includes(searchTerm.toLowerCase()) ||
        app.description.short.toLowerCase().includes(searchTerm.toLowerCase()) ||
        app.tags.some(t => t.toLowerCase().includes(searchTerm.toLowerCase()));
      return domainMatch && searchMatch;
    });
  }, [apps, selectedDomain, searchTerm]);

  const officialApps = filteredApps.filter(a => a.tier === 'official');
  const communityApps = filteredApps.filter(a => a.tier === 'community');

  return (
    <DefaultLayoutTemplate
      title="App Catalog"
      breadcrumbs={[]}
      subTitle="Choose an application environment for your experiment"
    >
      <Container padding="xlarge">
        <Loader loading={loading}>
          <Layout.Horizontal spacing="large" className={css.catalogLayout}>
            <Layout.Vertical spacing="small" className={css.sidebar}>
              <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>Filter by Domain</Text>
              {DOMAINS.map(d => (
                <div
                  key={d.id}
                  className={`${css.domainFilter} ${selectedDomain === d.id ? css.domainFilterActive : ''}`}
                  onClick={() => setSelectedDomain(d.id)}
                >
                  <Text
                    font={{ variation: FontVariation.BODY }}
                    color={selectedDomain === d.id ? Color.PRIMARY_7 : Color.GREY_700}
                  >
                    {d.displayName}
                  </Text>
                </div>
              ))}
            </Layout.Vertical>

            <Layout.Vertical spacing="large" className={css.mainContent}>
              <TextInput
                leftIcon="search"
                placeholder="Search apps..."
                value={searchTerm}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSearchTerm(e.target.value)}
                className={css.searchBar}
              />

              {officialApps.length > 0 && (
                <Layout.Vertical spacing="medium">
                  <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>Official</Text>
                  <div className={css.appGrid}>
                    {officialApps.map(app => <AppCard key={app.name} app={app} />)}
                  </div>
                </Layout.Vertical>
              )}

              {communityApps.length > 0 && (
                <Layout.Vertical spacing="medium">
                  <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_700}>Community</Text>
                  <div className={css.appGrid}>
                    {communityApps.map(app => <AppCard key={app.name} app={app} />)}
                  </div>
                </Layout.Vertical>
              )}

              {filteredApps.length === 0 && !loading && (
                <Layout.Vertical flex={{ justifyContent: 'center', alignItems: 'center' }} height={300} spacing="medium">
                  <Icon name="nav-settings" size={48} color={Color.GREY_400} />
                  <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>No apps found for this filter</Text>
                </Layout.Vertical>
              )}

              <Layout.Horizontal
                className={css.contributeBanner}
                flex={{ justifyContent: 'space-between', alignItems: 'center' }}
                padding="medium"
              >
                <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                  Don't see your domain? Contribute an app to the catalog.
                </Text>
                <Button
                  variation={ButtonVariation.SECONDARY}
                  text="Contribute an App"
                  icon="plus"
                  onClick={() => history.push(paths.toAppsOnboarding())}
                />
              </Layout.Horizontal>
            </Layout.Vertical>
          </Layout.Horizontal>
        </Loader>
      </Container>
    </DefaultLayoutTemplate>
  );
}
