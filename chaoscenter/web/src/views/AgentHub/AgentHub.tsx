import type { ApolloQueryResult } from '@apollo/client';
import { Color, FontVariation } from '@harnessio/design-system';
import { Card, Container, Layout, Text } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import React from 'react';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { AgentHubCategory, AgentHubEntry } from '@api/entities';
import type { ListAgentHubCategoriesRequest, ListAgentHubCategoriesResponse } from '@api/core';
import { useDocumentTitle } from '@hooks';
import { useStrings } from '@strings';
import Loader from '@components/Loader';
import css from './AgentHub.module.scss';

interface AgentHubViewProps {
  categories?: AgentHubCategory[];
  loading: boolean;
  refetch: (
    variables?: Partial<ListAgentHubCategoriesRequest> | undefined
  ) => Promise<ApolloQueryResult<ListAgentHubCategoriesResponse>>;
}

function DeploymentStatusBadge({ isDeployed, status }: { isDeployed: boolean; status: string }): React.ReactElement {
  const { getString } = useStrings();
  return (
    <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
      <svg width="8" height="8" viewBox="0 0 8 8" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="4" cy="4" r="4" fill={isDeployed ? '#0AB000' : '#999999'} />
      </svg>
      <Text font={{ variation: FontVariation.SMALL }} color={isDeployed ? Color.GREEN_700 : Color.GREY_500}>
        {isDeployed ? status || getString('agentDeployed') : getString('agentNotDeployed')}
      </Text>
    </Layout.Horizontal>
  );
}

function AgentCard({ agent }: { agent: AgentHubEntry }): React.ReactElement {
  return (
    <Card className={css.agentCard} elevation={1}>
      <Layout.Vertical spacing="medium" padding="medium">
        <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
            <Icon name="chaos-scenario-builder" size={24} color={Color.PRIMARY_7} />
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
              {agent.displayName}
            </Text>
          </Layout.Horizontal>
          <DeploymentStatusBadge isDeployed={agent.isDeployed} status={agent.deploymentStatus} />
        </Layout.Horizontal>
        <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600} lineClamp={2}>
          {agent.description}
        </Text>
        <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
          <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_500}>
            v{agent.version}
          </Text>
          {agent.namespace && (
            <>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_400}>|</Text>
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                {agent.namespace}
              </Text>
            </>
          )}
        </Layout.Horizontal>
        {agent.capabilities && agent.capabilities.length > 0 && (
          <Layout.Horizontal spacing="xsmall" className={css.capabilities}>
            {agent.capabilities.slice(0, 3).map(cap => (
              <Text key={cap} font={{ variation: FontVariation.SMALL }} className={css.capabilityTag}>
                {cap}
              </Text>
            ))}
            {agent.capabilities.length > 3 && (
              <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                +{agent.capabilities.length - 3} more
              </Text>
            )}
          </Layout.Horizontal>
        )}
      </Layout.Vertical>
    </Card>
  );
}

export default function AgentHubView({
  categories,
  loading,
  refetch: _refetch
}: AgentHubViewProps): React.ReactElement {
  const { getString } = useStrings();

  useDocumentTitle(getString('agentHub'));

  return (
    <DefaultLayoutTemplate title={getString('agentHub')} breadcrumbs={[]} subTitle={getString('agentHubDescription')}>
      <Container padding="xlarge">
        <Loader loading={loading}>
          {categories && categories.length > 0 ? (
            <Layout.Vertical spacing="xlarge">
              {categories.map(category => (
                <Layout.Vertical key={category.categoryName} spacing="medium">
                  <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
                    {category.categoryName}
                  </Text>
                  <Layout.Horizontal spacing="medium" className={css.agentGrid}>
                    {category.agents.map(agent => (
                      <AgentCard key={agent.name} agent={agent} />
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
              <Icon name="chaos-scenario-builder" size={48} color={Color.GREY_400} />
              <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
                No agents available in the hub
              </Text>
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_400}>
                Agent charts will appear here once the Agent Hub is synced
              </Text>
            </Layout.Vertical>
          )}
        </Loader>
      </Container>
    </DefaultLayoutTemplate>
  );
}
