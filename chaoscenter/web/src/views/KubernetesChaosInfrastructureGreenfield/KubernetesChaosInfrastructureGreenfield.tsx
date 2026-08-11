import { Button, ButtonVariation, Layout, Text, useToaster } from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import React from 'react';
import { useParams } from 'react-router-dom';
import type { MutationFunction } from '@api/types';
import { DeploymentScopeOptions } from '@models';
import { useStrings } from '@strings';
import { downloadYamlAsFile, getFormattedFileName, getScope } from '@utils';
import type { StepData } from '@views/KubernetesChaosInfrastructureCreationModal/KubernetesChaosInfrastructureStepWizardConfiguration';
import CodeBlock from '@components/CodeBlock';
import { InfrastructureType } from '@api/entities';
import config from '@config';
import type {
  connectChaosInfraManifestModeResponse,
  connectChaosInfraRequest
} from '@api/core/infrastructures/connectChaosInfra';

interface KubernetesChaosInfrastructureGreenfieldViewProps {
  connectChaosInfrastructureMutation: MutationFunction<connectChaosInfraManifestModeResponse, connectChaosInfraRequest>;
  data: StepData;
  infraRegistered: boolean;
  setInfraRegistered: React.Dispatch<React.SetStateAction<boolean>>;
}

export default function KubernetesChaosInfrastructureGreenfieldView({
  connectChaosInfrastructureMutation,
  data,
  infraRegistered,
  setInfraRegistered
}: KubernetesChaosInfrastructureGreenfieldViewProps): React.ReactElement {
  const { environmentID } = useParams<{ environmentID: string }>();
  const { getString } = useStrings();
  const scope = getScope();
  const { showError, showSuccess } = useToaster();
  const [loading, setLoading] = React.useState<boolean>(false);
  // Set once registration completes — the JWT lets the manifest be fetched
  // straight from `/file/<token>.yaml`, so a host with kubectl access to the
  // target cluster can apply it directly without ever going through a browser
  // download + manual copy.
  const [applyOnHostUrl, setApplyOnHostUrl] = React.useState<string | undefined>();

  // Fetch the formatted file name for giving apply command
  const fileName = getFormattedFileName(data.value.name);
  const kubectlApplyFile = `kubectl apply -f ${fileName}-litmus-chaos-enable.yml`;

  return (
    <Layout.Vertical spacing="small">
      <Layout.Horizontal flex={{ alignItems: 'center' }} spacing="small">
        <CodeBlock text={kubectlApplyFile} isCopyButtonEnabled />
        <Button
          disabled={infraRegistered}
          loading={loading}
          onClick={() => {
            setLoading(true);
            setInfraRegistered(true);
            connectChaosInfrastructureMutation({
              variables: {
                projectID: scope.projectID,
                request: {
                  infraScope: data.value.infraScope,
                  name: data.value.name,
                  environmentID: environmentID,
                  description: data.value.description ?? undefined,
                  platformName: 'Kubernetes',
                  infraNamespace: data.value.chaosInfrastructureNamespace,
                  serviceAccount: data.value.serviceAccountName,
                  infraNsExists: data.value.infraScope === DeploymentScopeOptions.CLUSTER ? false : true,
                  infraSaExists: false,
                  skipSsl: data.value.skipSSLCheck,
                  tolerations: data.value.tolerationValues ?? undefined,
                  nodeSelector: data.value.nodeSelectorValues
                    ? `${data.value.nodeSelectorValues[0].key.trim()}=${data.value.nodeSelectorValues[0].value.trim()}`
                    : undefined,
                  tags: data.value.tags ?? undefined,
                  infrastructureType: InfrastructureType.KUBERNETES
                }
              },
              onCompleted: (result: connectChaosInfraManifestModeResponse) => {
                setLoading(false);
                showSuccess(getString('chaosInfrastructureSuccess'));
                setApplyOnHostUrl(`${config.restEndpoints.chaosManagerUri}/file/${result.registerInfra.token}.yaml`);
                downloadYamlAsFile(result.registerInfra.manifest, `${fileName}-litmus-chaos-enable.yml`);
              },
              onError: err => {
                showError(err.message);
              }
            });
          }}
          variation={ButtonVariation.PRIMARY}
          text={getString('download')}
        />
      </Layout.Horizontal>
      {applyOnHostUrl && (
        <Layout.Vertical spacing="xsmall">
          <Text font={{ variation: FontVariation.SMALL_SEMI }} color={Color.GREY_800}>
            {getString('applyManifestOnHostTitle')}
          </Text>
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
            {getString('applyManifestOnHostPrompt')}
          </Text>
          <CodeBlock text={`kubectl apply -f ${applyOnHostUrl}`} isCopyButtonEnabled />
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600}>
            {getString('applyManifestOnHostFallbackPrompt')}
          </Text>
        </Layout.Vertical>
      )}
    </Layout.Vertical>
  );
}
