import React, { useMemo } from 'react';
import { Button, ButtonVariation, Card, Container, Layout, Text } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Icon } from '@harnessio/icons';
import { useListAgentsForStudio } from '@api/core';
import type { AgentForStudio } from '@api/core';
import Loader from '@components/Loader';

interface SelectAgentProps {
  appName: string;
  appDomain: string;
  onBack: () => void;
  onSelect: (
    agentName: string,
    agentVersion: string,
    allowUserChoice: boolean,
    allowedModels: string[]
  ) => void;
}

function AgentRow({
  agent,
  onSelect
}: {
  agent: AgentForStudio;
  onSelect: () => void;
}): React.ReactElement {
  return (
    <Card elevation={1} style={{ marginBottom: 12 }}>
      <Layout.Horizontal
        flex={{ justifyContent: 'space-between', alignItems: 'center' }}
        padding="medium"
        spacing="large"
      >
        <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center' }}>
          <Icon name="chaos-scenario-builder" size={28} color={Color.PRIMARY_7} />
          <Layout.Vertical spacing="xsmall">
            <Layout.Horizontal spacing="small" flex={{ alignItems: 'center' }}>
              <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
                {agent.displayName ?? agent.name}
              </Text>
              <Text font={{ variation: FontVariation.SMALL_BOLD }} color={Color.GREY_400}>
                v{agent.version}
              </Text>
              <Text
                font={{ variation: FontVariation.SMALL_BOLD }}
                style={{
                  background: '#e3f2fd',
                  color: '#1565c0',
                  borderRadius: 3,
                  padding: '1px 6px'
                }}
              >
                Compatible
              </Text>
            </Layout.Horizontal>

            {agent.description && (
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600} lineClamp={2}>
                {agent.description}
              </Text>
            )}

            <Layout.Horizontal spacing="medium">
              {agent.llmConfig && (
                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                  {agent.llmConfig.llmDependent ? 'LLM-dependent' : 'Rule-based'}
                  {agent.llmConfig.model ? ` · ${agent.llmConfig.model}` : ''}
                </Text>
              )}
              {agent.capabilities && agent.capabilities.length > 0 && (
                <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                  {agent.capabilities.length} capabilities
                </Text>
              )}
            </Layout.Horizontal>
          </Layout.Vertical>
        </Layout.Horizontal>

        <Button
          variation={ButtonVariation.PRIMARY}
          text="Select"
          onClick={onSelect}
          style={{ minWidth: 100 }}
        />
      </Layout.Horizontal>
    </Card>
  );
}

export default function SelectAgent({
  appName,
  appDomain,
  onBack,
  onSelect
}: SelectAgentProps): React.ReactElement {
  const { data, loading } = useListAgentsForStudio({
    variables: { pagination: { page: 1, limit: 100 } },
    fetchPolicy: 'cache-and-network'
  });

  const allAgents = data?.listAgents?.agents ?? [];

  // Filter agents compatible with the selected app:
  // Compatible means supportedApps is empty (all apps) or contains this app's name.
  // Also exclude agents that explicitly list this app as unsupported.
  const compatibleAgents = useMemo(() => {
    return allAgents.filter(agent => {
      const compat = agent.compatibility;
      if (!compat) return true; // no restriction = compatible
      if (compat.unsupportedApps?.includes(appName)) return false;
      if (!compat.supportedApps || compat.supportedApps.length === 0) return true;
      return compat.supportedApps.includes(appName);
    });
  }, [allAgents, appName]);

  return (
    <Container padding="xlarge">
      <Layout.Vertical spacing="large">
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
            Step 2 of 4: Select Agent
          </Text>
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
            App: <strong>{appName}</strong> ({appDomain})
            &nbsp;&mdash;&nbsp;Showing agents compatible with this application
          </Text>
        </Layout.Vertical>

        <Loader loading={loading}>
          {compatibleAgents.length > 0 ? (
            <Layout.Vertical spacing="small">
              {compatibleAgents.map(agent => (
                <AgentRow
                  key={agent.agentID}
                  agent={agent}
                  onSelect={() =>
                    onSelect(
                      agent.name,
                      agent.version,
                      agent.llmConfig?.allowUserChoice ?? false,
                      agent.llmConfig?.allowedModels ?? []
                    )
                  }
                />
              ))}
            </Layout.Vertical>
          ) : (
            <Layout.Vertical
              flex={{ justifyContent: 'center', alignItems: 'center' }}
              height={300}
              spacing="medium"
            >
              <Icon name="chaos-scenario-builder" size={48} color={Color.GREY_400} />
              <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
                No compatible agents found
              </Text>
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_400}>
                Register an agent that supports the <strong>{appName}</strong> application
              </Text>
              <Button
                variation={ButtonVariation.SECONDARY}
                text="Register a new agent"
                icon="plus"
                onClick={() => {
                  window.location.href = '/agent-onboarding';
                }}
              />
            </Layout.Vertical>
          )}
        </Loader>

        <Layout.Horizontal
          flex={{ justifyContent: 'flex-start' }}
          padding={{ top: 'medium' }}
        >
          <Button
            variation={ButtonVariation.TERTIARY}
            icon="chevron-left"
            text="Back"
            onClick={onBack}
          />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Container>
  );
}
