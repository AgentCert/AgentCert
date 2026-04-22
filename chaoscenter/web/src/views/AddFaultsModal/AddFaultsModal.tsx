import React, { useState, useMemo } from 'react';
import {
  Button,
  ButtonVariation,
  Checkbox,
  Container,
  ExpandingSearchInput,
  Layout,
  Text,
  useToaster
} from '@harnessio/uicore';
import { Color, FontVariation } from '@harnessio/design-system';
import { Dialog } from '@blueprintjs/core';
import { getScope } from '@utils';
import { listChaosFaults, updateFaultStudio } from '@api/core';
import type { Chart, FaultList, FaultStudio, FaultSelectionInput } from '@api/entities';
import Loader from '@components/Loader';
import css from './AddFaultsModal.module.scss';

interface FaultItem {
  category: string;
  categoryDisplayName: string;
  faultName: string;
  displayName: string;
  description: string;
}

interface AddFaultsModalProps {
  isOpen: boolean;
  onClose: () => void;
  faultStudio: FaultStudio;
  onSuccess: () => void;
}

export default function AddFaultsModal({
  isOpen,
  onClose,
  faultStudio,
  onSuccess
}: AddFaultsModalProps): React.ReactElement {
  const scope = getScope();
  const { showSuccess, showError } = useToaster();
  
  const [selectedFaults, setSelectedFaults] = useState<Set<string>>(new Set());
  const [searchTerm, setSearchTerm] = useState('');

  // Fetch faults from the source ChaosHub
  const { data: faultsData, loading: faultsLoading } = listChaosFaults({
    projectID: scope.projectID,
    hubID: faultStudio.sourceHubId,
    options: {
      skip: !isOpen || !faultStudio.sourceHubId
    }
  });

  // Update mutation
  const [updateFaultStudioMutation, { loading: updateLoading }] = updateFaultStudio();

  // Transform Chart data into flat fault list
  const allFaults: FaultItem[] = useMemo(() => {
    if (!faultsData?.listChaosFaults) return [];
    
    const faults: FaultItem[] = [];
    faultsData.listChaosFaults.forEach((chart: Chart) => {
      const category = chart.metadata.name;
      const categoryDisplayName = chart.spec.displayName;
      
      chart.spec.faults?.forEach((fault: FaultList) => {
        faults.push({
          category,
          categoryDisplayName,
          faultName: fault.name,
          displayName: fault.displayName,
          description: fault.description
        });
      });
    });
    
    return faults;
  }, [faultsData]);

  // Filter faults based on search
  const filteredFaults = useMemo(() => {
    if (!searchTerm) return allFaults;
    
    const term = searchTerm.toLowerCase();
    return allFaults.filter(fault => 
      fault.faultName.toLowerCase().includes(term) ||
      fault.displayName.toLowerCase().includes(term) ||
      fault.category.toLowerCase().includes(term) ||
      fault.categoryDisplayName.toLowerCase().includes(term)
    );
  }, [allFaults, searchTerm]);

  // Group filtered faults by category
  const faultsByCategory = useMemo(() => {
    const grouped: Record<string, { displayName: string; faults: FaultItem[] }> = {};
    
    filteredFaults.forEach(fault => {
      if (!grouped[fault.category]) {
        grouped[fault.category] = {
          displayName: fault.categoryDisplayName,
          faults: []
        };
      }
      grouped[fault.category].faults.push(fault);
    });
    
    return grouped;
  }, [filteredFaults]);

  // Get already added fault names
  const existingFaultNames = useMemo(() => {
    return new Set(faultStudio.selectedFaults?.map(f => `${f.faultCategory}/${f.faultName}`) || []);
  }, [faultStudio.selectedFaults]);

  const toggleFault = (category: string, faultName: string): void => {
    const key = `${category}/${faultName}`;
    const newSelected = new Set(selectedFaults);
    
    if (newSelected.has(key)) {
      newSelected.delete(key);
    } else {
      newSelected.add(key);
    }
    
    setSelectedFaults(newSelected);
  };

  const handleAddFaults = async (): Promise<void> => {
    if (selectedFaults.size === 0) return;

    try {
      // Convert selected faults to FaultSelectionInput
      const newFaults: FaultSelectionInput[] = Array.from(selectedFaults).map(key => {
        const [category, faultName] = key.split('/');
        const faultItem = allFaults.find(f => f.category === category && f.faultName === faultName);
        
        return {
          faultCategory: category,
          faultName: faultName,
          displayName: faultItem?.displayName || faultName,
          description: faultItem?.description,
          enabled: true,
          weight: 1
        };
      });

      // Merge with existing faults
      const existingFaults: FaultSelectionInput[] = (faultStudio.selectedFaults || []).map(f => ({
        faultCategory: f.faultCategory,
        faultName: f.faultName,
        displayName: f.displayName,
        description: f.description,
        enabled: f.enabled,
        injectionConfig: f.injectionConfig ? {
          injectionType: f.injectionConfig.injectionType,
          schedule: f.injectionConfig.schedule,
          duration: f.injectionConfig.duration,
          targetSelector: f.injectionConfig.targetSelector,
          interval: f.injectionConfig.interval
        } : undefined,
        customParameters: f.customParameters,
        weight: f.weight
      }));

      const allSelectedFaults = [...existingFaults, ...newFaults];

      await updateFaultStudioMutation({
        variables: {
          projectID: scope.projectID,
          studioID: faultStudio.id,
          request: {
            selectedFaults: allSelectedFaults
          }
        }
      });

      showSuccess(`${newFaults.length} fault(s) added successfully`);
      setSelectedFaults(new Set());
      onSuccess();
      onClose();
    } catch (error: unknown) {
      if (error instanceof Error) {
        showError(error.message);
      } else {
        showError('Failed to add faults');
      }
    }
  };

  const isLoading = faultsLoading || updateLoading;

  return (
    <Dialog
      isOpen={isOpen}
      onClose={onClose}
      title=""
      className={css.modal}
      canOutsideClickClose={false}
      canEscapeKeyClose={!updateLoading}
    >
      <Layout.Vertical className={css.modalContent}>
        {/* Header */}
        <Layout.Horizontal 
          flex={{ justifyContent: 'space-between', alignItems: 'center' }}
          padding={{ left: 'xlarge', right: 'xlarge', top: 'large' }}
        >
          <Layout.Vertical spacing="xsmall">
            <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
              Add Faults to Studio
            </Text>
            <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
              Select faults from &quot;{faultStudio.sourceHubName}&quot; to add to your Fault Studio
            </Text>
          </Layout.Vertical>
          <Button
            icon="cross"
            variation={ButtonVariation.ICON}
            onClick={onClose}
            disabled={updateLoading}
          />
        </Layout.Horizontal>

        {/* Search */}
        <Container padding={{ left: 'xlarge', right: 'xlarge', top: 'medium' }}>
          <ExpandingSearchInput
            alwaysExpanded
            placeholder="Search faults..."
            onChange={text => setSearchTerm(text.trim())}
            defaultValue={searchTerm}
            width="100%"
          />
        </Container>

        {/* Selected Count */}
        {selectedFaults.size > 0 && (
          <Container padding={{ left: 'xlarge', right: 'xlarge', top: 'small' }}>
            <Text font={{ variation: FontVariation.SMALL }} color={Color.PRIMARY_7}>
              {selectedFaults.size} fault(s) selected
            </Text>
          </Container>
        )}

        {/* Faults List */}
        <Container className={css.faultsList} padding={{ left: 'xlarge', right: 'xlarge', top: 'medium' }}>
          <Loader loading={faultsLoading}>
            {Object.keys(faultsByCategory).length === 0 ? (
              <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                {searchTerm ? 'No faults match your search' : 'No faults available in this ChaosHub'}
              </Text>
            ) : (
              <Layout.Vertical spacing="large">
                {Object.entries(faultsByCategory).map(([category, { displayName, faults }]) => (
                  <Layout.Vertical key={category} spacing="small">
                    <Text font={{ variation: FontVariation.H6 }} color={Color.GREY_700}>
                      {displayName || category} ({faults.length})
                    </Text>
                    <Layout.Vertical spacing="xsmall" padding={{ left: 'small' }}>
                      {faults.map(fault => {
                        const key = `${fault.category}/${fault.faultName}`;
                        const isAlreadyAdded = existingFaultNames.has(key);
                        const isSelected = selectedFaults.has(key);
                        
                        return (
                          <Layout.Horizontal 
                            key={key}
                            className={css.faultRow}
                            flex={{ alignItems: 'center', justifyContent: 'flex-start' }}
                            spacing="small"
                          >
                            <Checkbox
                              checked={isSelected}
                              disabled={isAlreadyAdded || updateLoading}
                              onChange={() => toggleFault(fault.category, fault.faultName)}
                            />
                            <Layout.Vertical spacing="none">
                              <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
                                <Text 
                                  font={{ variation: FontVariation.BODY }} 
                                  color={isAlreadyAdded ? Color.GREY_400 : Color.GREY_800}
                                >
                                  {fault.displayName || fault.faultName}
                                </Text>
                                {isAlreadyAdded && (
                                  <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_400}>
                                    (already added)
                                  </Text>
                                )}
                              </Layout.Horizontal>
                              {fault.description && (
                                <Text 
                                  font={{ variation: FontVariation.SMALL }} 
                                  color={Color.GREY_400}
                                  lineClamp={1}
                                >
                                  {fault.description}
                                </Text>
                              )}
                            </Layout.Vertical>
                          </Layout.Horizontal>
                        );
                      })}
                    </Layout.Vertical>
                  </Layout.Vertical>
                ))}
              </Layout.Vertical>
            )}
          </Loader>
        </Container>

        {/* Actions */}
        <Layout.Horizontal 
          spacing="medium" 
          padding="xlarge"
          className={css.actions}
        >
          <Button
            variation={ButtonVariation.PRIMARY}
            text={updateLoading ? 'Adding...' : `Add ${selectedFaults.size} Fault(s)`}
            onClick={handleAddFaults}
            disabled={isLoading || selectedFaults.size === 0}
            icon={updateLoading ? 'loading' : undefined}
          />
          <Button
            variation={ButtonVariation.TERTIARY}
            text="Cancel"
            onClick={onClose}
            disabled={updateLoading}
          />
        </Layout.Horizontal>
      </Layout.Vertical>
    </Dialog>
  );
}
