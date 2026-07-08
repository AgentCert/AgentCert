import React, { useState, useMemo } from 'react';
import {
  Button,
  ButtonVariation,
  Card,
  Container,
  Layout,
  Text,
  TextInput
} from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Icon } from '@harnessio/icons';
import { listApplications } from '@api/core';
import { getScope } from '@utils';
import type { ApplicationSpec } from '@api/entities';
import Loader from '@components/Loader';

interface SelectAppProps {
  onSelect: (appName: string, appDomain: string, microservices: string[]) => void;
}

const DOMAIN_ALL = 'all';

function AppCard({
  app,
  onSelect
}: {
  app: ApplicationSpec;
  onSelect: () => void;
}): React.ReactElement {
  const compatibleFaults = app.faultCompatibility.filter(f => f.compatible).length;
  return (
    <Card
      elevation={1}
      interactive
      onClick={onSelect}
      style={{ cursor: 'pointer', minWidth: 240, maxWidth: 300 }}
    >
      <Layout.Vertical spacing="medium" padding="medium">
        <Layout.Horizontal
          flex={{ justifyContent: 'space-between', alignItems: 'flex-start' }}
        >
          <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
            <Icon name="nav-settings" size={20} color={Color.PRIMARY_7} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
              {app.displayName}
            </Text>
          </Layout.Horizontal>
          {app.tier === 'official' && (
            <Text
              font={{ variation: FontVariation.SMALL_BOLD }}
              style={{
                background: '#f5a623',
                color: '#fff',
                borderRadius: 3,
                padding: '2px 6px'
              }}
            >
              Official
            </Text>
          )}
        </Layout.Horizontal>

        <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
          <Text
            font={{ variation: FontVariation.SMALL_BOLD }}
            style={{
              background: Color.PRIMARY_1,
              color: Color.PRIMARY_7,
              borderRadius: 3,
              padding: '1px 6px'
            }}
          >
            {app.domain}
          </Text>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_400}>
            v{app.version}
          </Text>
        </Layout.Horizontal>

        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600} lineClamp={2}>
          {app.description.short}
        </Text>

        <Layout.Horizontal spacing="medium">
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            {app.microservices.length} microservices
          </Text>
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            {compatibleFaults} compatible faults
          </Text>
        </Layout.Horizontal>

        <Button
          variation={ButtonVariation.PRIMARY}
          text="Select"
          small
          onClick={e => {
            e.stopPropagation();
            onSelect();
          }}
        />
      </Layout.Vertical>
    </Card>
  );
}

export default function SelectApp({ onSelect }: SelectAppProps): React.ReactElement {
  const scope = getScope();
  const [selectedDomain, setSelectedDomain] = useState<string>(DOMAIN_ALL);
  const [searchTerm, setSearchTerm] = useState<string>('');

  const { data, loading } = listApplications({
    variables: { projectID: scope.projectID },
    fetchPolicy: 'cache-and-network'
  });

  const allApps = data?.listApplications ?? [];

  const domains = useMemo<string[]>(() => {
    const domainSet = new Set(allApps.map(a => a.domain));
    return Array.from(domainSet).sort();
  }, [allApps]);

  const filteredApps = useMemo(() => {
    return allApps.filter(app => {
      const domainMatch = selectedDomain === DOMAIN_ALL || app.domain === selectedDomain;
      const searchMatch =
        searchTerm === '' ||
        app.displayName.toLowerCase().includes(searchTerm.toLowerCase()) ||
        app.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        app.tags.some(t => t.toLowerCase().includes(searchTerm.toLowerCase()));
      return domainMatch && searchMatch;
    });
  }, [allApps, selectedDomain, searchTerm]);

  return (
    <Container padding="xlarge">
      <Layout.Vertical spacing="large">
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
            Step 1 of 4: Select Application
          </Text>
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
            Choose the application to run chaos experiments against
          </Text>
        </Layout.Vertical>

        <Layout.Horizontal spacing="large">
          {/* Domain sidebar */}
          <Layout.Vertical spacing="small" style={{ minWidth: 180 }}>
            <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
              Filter by Domain
            </Text>
            {[{ id: DOMAIN_ALL, label: 'All Domains' }, ...domains.map(d => ({ id: d, label: d }))].map(
              dom => (
                <div
                  key={dom.id}
                  onClick={() => setSelectedDomain(dom.id)}
                  style={{
                    padding: '6px 12px',
                    borderRadius: 4,
                    cursor: 'pointer',
                    backgroundColor: selectedDomain === dom.id ? '#e8f0fe' : 'transparent',
                    fontWeight: selectedDomain === dom.id ? 600 : 400
                  }}
                >
                  <Text
                    font={{ variation: FontVariation.BODY }}
                    color={selectedDomain === dom.id ? Color.PRIMARY_7 : Color.GREY_700}
                  >
                    {dom.label}
                  </Text>
                </div>
              )
            )}
          </Layout.Vertical>

          {/* App grid */}
          <Layout.Vertical spacing="medium" style={{ flex: 1 }}>
            <TextInput
              leftIcon="search"
              placeholder="Search apps by name or tag..."
              value={searchTerm}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSearchTerm(e.target.value)}
            />

            <Loader loading={loading}>
              {filteredApps.length > 0 ? (
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
                    gap: 16
                  }}
                >
                  {filteredApps.map(app => (
                    <AppCard
                      key={app.name}
                      app={app}
                      onSelect={() =>
                        onSelect(
                          app.name,
                          app.domain,
                          app.microservices.map(ms => ms.name)
                        )
                      }
                    />
                  ))}
                </div>
              ) : (
                <Layout.Vertical
                  flex={{ justifyContent: 'center', alignItems: 'center' }}
                  height={300}
                  spacing="medium"
                >
                  <Icon name="nav-settings" size={48} color={Color.GREY_400} />
                  <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
                    No apps match the current filter
                  </Text>
                </Layout.Vertical>
              )}
            </Loader>
          </Layout.Vertical>
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
