import type { ApolloQueryResult } from '@apollo/client';
import { Classes, Intent, Menu, PopoverInteractionKind, Position, Switch } from '@blueprintjs/core';
import { Color, FontVariation } from '@harnessio/design-system';
import { Container, Layout, Text, Tabs, Tab, Button, ButtonVariation, Card, CardBody, useToggleOpen, useToaster, ConfirmationDialog } from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import React, { useState } from 'react';
import { useHistory } from 'react-router-dom';
import { getDetailedTime, getScope, killEvent } from '@utils';
import { useStrings } from '@strings';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { FaultStudio, FaultSelection } from '@api/entities';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import Loader from '@components/Loader';
import { EditFaultStudioModalController } from '@controllers/EditFaultStudioModal';
import DeleteFaultStudioDialog from '@components/DeleteFaultStudioDialog';
import AddFaultsModal from '@views/AddFaultsModal';
import RbacMenuItem from '@components/RbacMenuItem';
import { PermissionGroup } from '@models';
import { removeFaultFromStudio, toggleFaultInStudio } from '@api/core';
import css from './FaultStudio.module.scss';

interface FaultStudioViewProps {
  faultStudio?: FaultStudio;
  loading: boolean;
  refetch: () => Promise<ApolloQueryResult<unknown>>;
}

interface FaultCardProps {
  fault: FaultSelection;
  onRemove: (faultName: string) => void;
  onToggle: (faultName: string, enabled: boolean) => void;
  isToggling?: boolean;
}

function FaultCard({ fault, onRemove, onToggle, isToggling }: FaultCardProps): React.ReactElement {
  const handleToggleClick = (e: React.MouseEvent): void => {
    e.stopPropagation();
    onToggle(fault.faultName, !fault.enabled);
  };

  return (
    <Card className={css.faultCard}>
      <Layout.Vertical spacing="small">
        <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_800}>
            {fault.displayName || fault.faultName}
          </Text>
          <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
            <div onClick={handleToggleClick} style={{ cursor: 'pointer' }}>
              <Switch
                checked={fault.enabled}
                disabled={isToggling}
                className={css.faultToggle}
              />
            </div>
            <div onClick={killEvent}>
              <CardBody.Menu
                menuPopoverProps={{
                  className: Classes.DARK,
                  position: Position.RIGHT,
                  interactionKind: PopoverInteractionKind.HOVER
                }}
                menuContent={
                  <Menu>
                    <RbacMenuItem
                      icon="trash"
                      text="Remove"
                      onClick={() => onRemove(fault.faultName)}
                      permission={PermissionGroup.OWNER}
                    />
                  </Menu>
                }
              />
            </div>
          </Layout.Horizontal>
        </Layout.Horizontal>
        <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
          Category: {fault.faultCategory}
        </Text>
        {fault.injectionConfig && (
          <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
            Injection: {fault.injectionConfig.injectionType}
            {fault.injectionConfig.duration && ` - ${fault.injectionConfig.duration}`}
          </Text>
        )}
        {fault.weight !== undefined && (
          <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_400}>
            Weight: {fault.weight}
          </Text>
        )}
      </Layout.Vertical>
    </Card>
  );
}

export default function FaultStudioView({
  faultStudio,
  loading,
  refetch
}: FaultStudioViewProps): React.ReactElement {
  const { getString } = useStrings();
  const paths = useRouteWithBaseUrl();
  const history = useHistory();
  const scope = getScope();
  const { showSuccess, showError } = useToaster();
  
  // Modal state
  const { isOpen: isEditModalOpen, open: openEditModal, close: closeEditModal } = useToggleOpen();
  const { isOpen: isDeleteDialogOpen, open: openDeleteDialog, close: closeDeleteDialog } = useToggleOpen();
  const { isOpen: isAddFaultsModalOpen, open: openAddFaultsModal, close: closeAddFaultsModal } = useToggleOpen();
  
  // Remove fault state
  const [faultToRemove, setFaultToRemove] = useState<string | null>(null);
  
  // Toggle fault state
  const [togglingFault, setTogglingFault] = useState<string | null>(null);
  
  // Remove fault mutation
  const [removeFaultMutation, { loading: removingFault }] = removeFaultFromStudio({
    onCompleted: () => {
      showSuccess('Fault removed successfully');
      setFaultToRemove(null);
      refetch();
    },
    onError: (err) => {
      showError(err.message);
      setFaultToRemove(null);
    }
  });
  
  // Toggle fault mutation
  const [toggleFaultMutation] = toggleFaultInStudio({
    onCompleted: (data) => {
      const newState = data.toggleFaultInStudio.message?.includes('enabled') ? 'enabled' : 'disabled';
      showSuccess(`Fault ${newState} successfully`);
      setTogglingFault(null);
      refetch();
    },
    onError: (err) => {
      showError(err.message);
      setTogglingFault(null);
    }
  });
  
  useDocumentTitle(faultStudio?.name || getString('faultStudio'));

  const breadcrumbs = [
    { label: getString('faultStudios'), url: paths.toFaultStudios() },
    { label: faultStudio?.name || '', url: '' }
  ];

  const handleEditSuccess = (): void => {
    refetch();
  };

  const handleDeleteSuccess = (): void => {
    // Navigate back to list after deletion
    history.push(paths.toFaultStudios());
  };

  const handleAddFaultsSuccess = (): void => {
    refetch();
  };

  const handleRemoveFault = (faultName: string): void => {
    setFaultToRemove(faultName);
  };

  const confirmRemoveFault = (): void => {
    if (faultToRemove && faultStudio) {
      removeFaultMutation({
        variables: {
          projectID: scope.projectID,
          studioID: faultStudio.id,
          faultName: faultToRemove
        }
      });
    }
  };

  const handleToggleFault = (faultName: string, enabled: boolean): void => {
    if (faultStudio) {
      setTogglingFault(faultName);
      toggleFaultMutation({
        variables: {
          projectID: scope.projectID,
          studioID: faultStudio.id,
          faultName,
          enabled
        }
      });
    }
  };

  const HeaderToolbar = (
    <Layout.Horizontal spacing="small">
      <Button
        variation={ButtonVariation.PRIMARY}
        text="+ Add Faults"
        icon="plus"
        onClick={openAddFaultsModal}
        disabled={!faultStudio}
      />
      <Button
        variation={ButtonVariation.SECONDARY}
        text={getString('edit')}
        icon="edit"
        onClick={openEditModal}
        disabled={!faultStudio}
      />
      <Button
        variation={ButtonVariation.TERTIARY}
        text={faultStudio?.isActive ? 'Deactivate' : 'Activate'}
        onClick={() => {
          // TODO: Toggle active state
        }}
      />
      <Button
        variation={ButtonVariation.TERTIARY}
        text={getString('delete')}
        icon="trash"
        onClick={openDeleteDialog}
        disabled={!faultStudio}
      />
    </Layout.Horizontal>
  );

  return (
    <DefaultLayoutTemplate
      title={faultStudio?.name || getString('faultStudio')}
      breadcrumbs={breadcrumbs}
      headerToolbar={HeaderToolbar}
    >
      <Loader loading={loading}>
        {faultStudio ? (
          <Container padding="medium">
            <Layout.Vertical spacing="large">
              {/* Studio Info Card */}
              <Card className={css.infoCard}>
                <Layout.Vertical spacing="medium">
                  <Layout.Horizontal flex={{ justifyContent: 'space-between' }}>
                    <Text font={{ variation: FontVariation.H4 }} color={Color.GREY_800}>
                      {faultStudio.name}
                    </Text>
                    <div className={css.activeStatus}>
                      <svg width="8" height="8" viewBox="0 0 8 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <circle cx="4" cy="4" r="4" fill={faultStudio.isActive ? '#0AB000' : '#CF2318'} />
                      </svg>
                      <Text font={{ variation: FontVariation.BODY }} color={faultStudio.isActive ? Color.GREEN_700 : Color.RED_600}>
                        {faultStudio.isActive ? 'Active' : 'Inactive'}
                      </Text>
                    </div>
                  </Layout.Horizontal>
                  {faultStudio.description && (
                    <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_600}>
                      {faultStudio.description}
                    </Text>
                  )}
                  <Layout.Horizontal spacing="large">
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                      <Icon name="layers" size={12} /> Source: {faultStudio.sourceHubName}
                    </Text>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                      <Icon name="folder-open" size={12} /> {faultStudio.totalFaults} faults
                    </Text>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                      <Icon name="tick" size={12} /> {faultStudio.enabledFaults} enabled
                    </Text>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                      <Icon name="time" size={12} /> Created: {getDetailedTime(parseInt(faultStudio.createdAt, 10))}
                    </Text>
                    <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                      <Icon name="refresh" size={12} /> Updated: {getDetailedTime(parseInt(faultStudio.updatedAt, 10))}
                    </Text>
                  </Layout.Horizontal>
                </Layout.Vertical>
              </Card>

              {/* Faults Section */}
              <Tabs id="fault-studio-tabs">
                <Tab
                  id="faults"
                  title={`Faults (${faultStudio.selectedFaults?.length || 0})`}
                  panel={
                    <Container padding={{ top: 'medium' }}>
                      {faultStudio.selectedFaults && faultStudio.selectedFaults.length > 0 ? (
                        <Layout.Vertical spacing="medium">
                          <Layout.Horizontal spacing="medium" className={css.faultsContainer}>
                            {faultStudio.selectedFaults.map((fault: FaultSelection, index: number) => (
                              <FaultCard 
                                key={`${fault.faultName}-${index}`} 
                                fault={fault} 
                                onRemove={handleRemoveFault}
                                onToggle={handleToggleFault}
                                isToggling={togglingFault === fault.faultName}
                              />
                            ))}
                          </Layout.Horizontal>
                        </Layout.Vertical>
                      ) : (
                        <Layout.Vertical spacing="medium" flex={{ alignItems: 'center' }} padding="xlarge">
                          <Icon name="add" size={48} color={Color.GREY_400} />
                          <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                            No faults configured yet
                          </Text>
                          <Button
                            variation={ButtonVariation.PRIMARY}
                            text="+ Add Faults"
                            icon="plus"
                            onClick={openAddFaultsModal}
                          />
                        </Layout.Vertical>
                      )}
                    </Container>
                  }
                />
                <Tab
                  id="settings"
                  title="Settings"
                  panel={
                    <Container padding={{ top: 'medium' }}>
                      <Text font={{ variation: FontVariation.BODY }} color={Color.GREY_500}>
                        Studio settings will appear here.
                      </Text>
                    </Container>
                  }
                />
              </Tabs>
            </Layout.Vertical>
          </Container>
        ) : (
          <Container padding="xlarge">
            <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_500}>
              Fault Studio not found
            </Text>
          </Container>
        )}
      </Loader>
      
      {/* Edit Fault Studio Modal */}
      {faultStudio && (
        <EditFaultStudioModalController
          isOpen={isEditModalOpen}
          onClose={closeEditModal}
          faultStudio={faultStudio}
          onSuccess={handleEditSuccess}
        />
      )}
      
      {/* Delete Fault Studio Confirmation Dialog */}
      {faultStudio && (
        <DeleteFaultStudioDialog
          isOpen={isDeleteDialogOpen}
          onClose={closeDeleteDialog}
          studioId={faultStudio.id}
          studioName={faultStudio.name}
          onSuccess={handleDeleteSuccess}
        />
      )}
      
      {/* Add Faults Modal */}
      {faultStudio && (
        <AddFaultsModal
          isOpen={isAddFaultsModalOpen}
          onClose={closeAddFaultsModal}
          faultStudio={faultStudio}
          onSuccess={handleAddFaultsSuccess}
        />
      )}
      
      {/* Remove Fault Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={faultToRemove !== null}
        titleText="Remove Fault"
        contentText={`Are you sure you want to remove the fault "${faultToRemove}" from this studio?`}
        confirmButtonText={removingFault ? 'Removing...' : 'Remove'}
        cancelButtonText="Cancel"
        intent={Intent.DANGER}
        onClose={(isConfirmed: boolean) => {
          if (isConfirmed) {
            confirmRemoveFault();
          } else {
            setFaultToRemove(null);
          }
        }}
      />
    </DefaultLayoutTemplate>
  );
}
