import { Color, FontVariation } from '@harnessio/design-system';
import { Card, Container, Layout, Text } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import React from 'react';
import { useParams } from 'react-router-dom';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { AgentHubCategory, AgentHubEntry } from '@api/entities';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import { useStrings } from '@strings';
import css from './AgentDetail.module.scss';

interface AgentDetailViewProps {
  categories?: AgentHubCategory[];
  loading: boolean;
}

export default function AgentDetailView({ categories, loading }: AgentDetailViewProps): React.ReactElement {
  const { getString } = useStrings();
  const paths = useRouteWithBaseUrl();
  const { agentName } = useParams<{ agentName: string }>();

  const agent: AgentHubEntry | undefined = React.useMemo(() => {
    if (!categories) return undefined;
    for (const cat of categories) {
      const found = cat.agents.find(a => a.name === agentName);
      if (found) return found;
    }
    return undefined;
  }, [categories, agentName]);

  useDocumentTitle(agent?.displayName ?? getString('agentHub'));

  const breadcrumbs = [
    { label: getString('agentHub'), url: paths.toAgentHub() }
  ];

  if (loading) {
    return (
      <DefaultLayoutTemplate title={getString('agentHub')} breadcrumbs={breadcrumbs}>
        <Container padding="xlarge">
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>Loading...</Text>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  if (!agent) {
    return (
      <DefaultLayoutTemplate title={getString('agentHub')} breadcrumbs={breadcrumbs}>
        <Container padding="xlarge">
          <Layout.Vertical flex={{ justifyContent: 'center', alignItems: 'center' }} height={400} spacing="medium">
            <Icon name="chaos-scenario-builder" size={48} color={Color.GREY_400} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
              Agent not found
            </Text>
          </Layout.Vertical>
        </Container>
      </DefaultLayoutTemplate>
    );
  }

  return (
    <DefaultLayoutTemplate title={agent.displayName} breadcrumbs={breadcrumbs}>
      <Container padding="xlarge" className={css.container}>
        <Card className={css.detailCard} elevation={1}>
          <Layout.Vertical spacing="large" padding="xlarge">
            {/* Header */}
            <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center' }}>
              <Icon name="chaos-scenario-builder" size={36} color={Color.PRIMARY_7} />
              <Layout.Vertical spacing="xsmall">
                <Text font={{ variation: FontVariation.H3 }} color={Color.GREY_800}>
                  {agent.displayName}
                </Text>
                <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_500}>
                  v{agent.version}
                </Text>
              </Layout.Vertical>
            </Layout.Horizontal>

            {/* Description */}
            <Layout.Vertical spacing="xsmall" className={css.section}>
              <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>Description</Text>
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_700}>
                {agent.description}
              </Text>
            </Layout.Vertical>

            {/* Details */}
            <Layout.Vertical spacing="small" className={css.section}>
              <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>Details</Text>
              <Layout.Horizontal spacing="xlarge">
                <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Name</Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{agent.name}</Text>
                </Layout.Vertical>
                <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                  <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Version</Text>
                  <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>v{agent.version}</Text>
                </Layout.Vertical>
                {agent.namespace && (
                  <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Namespace</Text>
                    <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{agent.namespace}</Text>
                  </Layout.Vertical>
                )}
                {agent.helmReleaseName && (
                  <Layout.Vertical spacing="xsmall" className={css.infoRow}>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>Helm Release</Text>
                    <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>{agent.helmReleaseName}</Text>
                  </Layout.Vertical>
                )}
              </Layout.Horizontal>
            </Layout.Vertical>

            {/* Capabilities */}
            {agent.capabilities && agent.capabilities.length > 0 && (
              <Layout.Vertical spacing="small">
                <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_600}>Capabilities</Text>
                <Layout.Horizontal spacing="xsmall" className={css.capabilities}>
                  {agent.capabilities.map(cap => (
                    <Text key={cap} font={{ variation: FontVariation.SMALL }} className={css.capabilityTag}>
                      {cap}
                    </Text>
                  ))}
                </Layout.Horizontal>
              </Layout.Vertical>
            )}
          </Layout.Vertical>
        </Card>
      </Container>
    </DefaultLayoutTemplate>
  );
}
