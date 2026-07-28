import React from 'react';
import { Button, Container, Layout, Text, useToaster } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import { Color, FontVariation } from '@harnessio/design-system';
import { parse } from 'yaml';
import { useParams } from 'react-router-dom';
import { useStrings } from '@strings';
import VisualizeExperimentManifestView from '@views/VisualizeExperimentManifest';
import { FaultList, InfrastructureType, PredefinedExperiment } from '@api/entities';
import type { ExperimentManifest } from '@models';
import experimentYamlService from 'services/experiment';
import type { CustomizedMultiSelectOption } from '@controllers/ListChaosHubsTab';
import config from '@config';
import { getScope, toTitleCase } from '@utils';
import css from './ListPredefinedExperiments.module.scss';

interface ListPredefinedExperimentsViewProps {
  predefinedExperiments: PredefinedExperiment[] | undefined;
  hub: CustomizedMultiSelectOption;
  onClose: (manifest: ExperimentManifest) => void;
}
interface HubCardProps {
  hub: CustomizedMultiSelectOption;
  manifest: string;
  csv: string;
  onClose: (manifest: ExperimentManifest) => void;
}

export function PredefinedExperimentCard({ manifest, csv, onClose, hub }: HubCardProps): React.ReactElement {
  const [experimentExpanded, setFaultExpanded] = React.useState<boolean>(false);
  const { getString } = useStrings();
  const scope = getScope();
  const parsedCSV = parse(csv);
  const parsedManifest = parse(manifest) as ExperimentManifest;
  const { showError } = useToaster();
  const { experimentKey } = useParams<{ experimentKey: string }>();
  const experimentHandler = experimentYamlService.getInfrastructureTypeHandler(InfrastructureType.KUBERNETES);

  const handleSelect = (event: React.MouseEvent<Element, MouseEvent>): void => {
    event.preventDefault();
    experimentHandler?.getExperiment(experimentKey).then(experiment => {
      const processedManifest = experimentHandler.preProcessExperimentManifest({
        manifest: parsedManifest,
        experimentName: experiment?.name ?? 'chaos-experiment',
        chaosInfrastructureID: experiment?.chaosInfrastructure?.id,
        chaosInfrastructureNamespace: experiment?.chaosInfrastructure?.namespace,
        imageRegistry: experiment?.imageRegistry
      });
      experimentHandler?.updateExperimentManifest(experimentKey, processedManifest);
      onClose(processedManifest);
    });
  };

  const handleExpanded = (trigger: boolean): void => {
    if (!manifest) showError(getString('manifestMissing'));
    else setFaultExpanded(trigger);
  };

  return (
    <div className={css.hubCard}>
      <div className={css.hubCardHeader} onClick={() => handleExpanded(!experimentExpanded)}>
        <img
          src={`${config.restEndpoints?.chaosManagerUri}/icon/${hub?.isDefault ? 'default' : scope.projectID}/${
            hub?.label
          }/predefined/${parsedCSV.metadata.name}.png`}
          height={25}
          width={25}
          alt={parsedCSV.metadata.name}
        />
        <div>
          <Text font={{ variation: FontVariation.BODY, weight: 'semi-bold' }}>{parsedCSV.spec.displayName}</Text>
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_600} margin={{ top: 'xsmall' }}>
            {parsedCSV.metadata.annotations.chartDescription}
          </Text>
        </div>
      </div>
      {experimentExpanded && parsedCSV.spec.faults && (
        <Container padding="medium" background={Color.PRIMARY_BG}>
          <Layout.Horizontal flex margin={{ bottom: 'small' }}>
            <Text font={{ variation: FontVariation.SMALL, weight: 'semi-bold' }} color={Color.GREY_600}>
              {getString('preview')}
            </Text>
            <Icon name="cross" size={20} onClick={() => setFaultExpanded(false)} color={Color.PRIMARY_7} />
          </Layout.Horizontal>
          <div className={css.previewCont}>
            <VisualizeExperimentManifestView manifest={parsedManifest} initialZoomLevel={0.6} />
          </div>
          <Text font={{ variation: FontVariation.BODY, weight: 'semi-bold' }}>{getString('faults')}</Text>
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
            {parsedCSV.spec.faults.map((experiment: FaultList) => experiment.name).join(', ')}
          </Text>
          <Button
            intent="primary"
            margin={{ top: 'medium' }}
            onClick={e => {
              handleSelect(e);
            }}
          >
            {getString('useThisTemplate')}
          </Button>
        </Container>
      )}
    </div>
  );
}

export default function ListPredefinedExperimentsView({
  predefinedExperiments,
  hub,
  onClose
}: ListPredefinedExperimentsViewProps): React.ReactElement {
  const { getString } = useStrings();
  const [activeCategory, setActiveCategory] = React.useState<string>('All');

  if (!predefinedExperiments || predefinedExperiments.length === 0) return <></>;

  const categoryCountMap = new Map<string, number>();
  predefinedExperiments.forEach(exp => {
    const category: string = parse(exp.experimentCSV)?.metadata?.annotations?.categories ?? 'Other';
    categoryCountMap.set(category, (categoryCountMap.get(category) ?? 0) + 1);
  });

  const filteredExperiments =
    activeCategory === 'All'
      ? predefinedExperiments
      : predefinedExperiments.filter(
          exp => (parse(exp.experimentCSV)?.metadata?.annotations?.categories ?? 'Other') === activeCategory
        );

  return (
    <Container padding={{ top: 'small', bottom: 'small' }}>
      <Text font={{ variation: FontVariation.H3, weight: 'semi-bold' }}>{hub?.label}</Text>
      <Layout.Horizontal margin={{ top: 'medium' }} className={css.mainLayout}>
        <Layout.Vertical width="200px" padding="medium" background={Color.WHITE} className={css.sidebar}>
          <Layout.Horizontal
            flex
            padding={{ left: 'small', right: 'small', top: 'medium', bottom: 'medium' }}
            className={`${css.tagCard} ${activeCategory === 'All' ? css.activeTag : ''}`}
            onClick={() => setActiveCategory('All')}
          >
            <Layout.Horizontal flex={{ alignItems: 'center', justifyContent: 'flex-start' }} className={css.gap1}>
              <Icon name="nav-settings" size={22} />
              <Text className={css.tagText} font={{ variation: FontVariation.SMALL }}>
                {getString('all')}
              </Text>
            </Layout.Horizontal>
            <Text
              font={{ variation: FontVariation.TINY }}
              color={activeCategory === 'All' ? Color.WHITE : Color.PRIMARY_7}
              background={activeCategory === 'All' ? Color.PRIMARY_7 : Color.PRIMARY_BG}
              height={25}
              width={25}
              flex={{ alignItems: 'center', justifyContent: 'center' }}
              className={css.rounded}
            >
              {predefinedExperiments.length}
            </Text>
          </Layout.Horizontal>
          {[...categoryCountMap.entries()].map(([category, count]) => (
            <Layout.Horizontal
              key={category}
              flex
              padding={{ left: 'small', right: 'small', top: 'medium', bottom: 'medium' }}
              className={`${css.tagCard} ${activeCategory === category ? css.activeTag : ''}`}
              onClick={() => setActiveCategory(category)}
            >
              <Text className={css.tagText} font={{ variation: FontVariation.SMALL }} lineClamp={1}>
                {toTitleCase({ text: category, separator: '-' })}
              </Text>
              <Text
                font={{ variation: FontVariation.TINY }}
                color={activeCategory === category ? Color.WHITE : Color.PRIMARY_7}
                background={activeCategory === category ? Color.PRIMARY_7 : Color.PRIMARY_BG}
                height={25}
                width={25}
                flex={{ alignItems: 'center', justifyContent: 'center' }}
                className={css.rounded}
              >
                {count}
              </Text>
            </Layout.Horizontal>
          ))}
        </Layout.Vertical>
        <Container className={css.experimentGrid}>
          {filteredExperiments.map(experiment => (
            <PredefinedExperimentCard
              hub={hub}
              key={experiment.experimentName}
              manifest={experiment.experimentManifest}
              csv={experiment.experimentCSV}
              onClose={onClose}
            />
          ))}
        </Container>
      </Layout.Horizontal>
    </Container>
  );
}
