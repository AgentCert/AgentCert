import type { ApolloQueryResult } from '@apollo/client';
import { Classes, Menu, PopoverInteractionKind, Position } from '@blueprintjs/core';
import { Color, FontVariation } from '@harnessio/design-system';
import {
  Card,
  CardBody,
  Container,
  ExpandingSearchInput,
  Layout,
  Text,
  Button,
  ButtonVariation,
  useToggleOpen
} from '@harnessio/uicore';
import { Icon } from '@harnessio/icons';
import React, { useState } from 'react';
import { useHistory } from 'react-router-dom';
import { getDetailedTime, killEvent } from '@utils';
import { useStrings } from '@strings';
import DefaultLayoutTemplate from '@components/DefaultLayout';
import type { FaultStudioSummary } from '@api/entities';
import { useDocumentTitle, useRouteWithBaseUrl } from '@hooks';
import NoFilteredData from '@components/NoFilteredData';
import Loader from '@components/Loader';
import { CreateFaultStudioModalController } from '@controllers/CreateFaultStudioModal';
import DeleteFaultStudioDialog from '@components/DeleteFaultStudioDialog';
import RbacMenuItem from '@components/RbacMenuItem';
import { PermissionGroup } from '@models';
import css from './FaultStudios.module.scss';

interface FaultStudiosViewProps {
  faultStudios: FaultStudioSummary[];
  totalCount: number;
  loading: boolean;
  searchTerm: string;
  setSearchTerm: React.Dispatch<React.SetStateAction<string>>;
  refetch: () => Promise<ApolloQueryResult<unknown>>;
}

function ActiveStatus({ isActive }: { isActive: boolean }): React.ReactElement {
  return (
    <div className={css.activeStatus}>
      <svg width="6" height="6" viewBox="0 0 6 6" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="3" cy="3" r="3" fill={isActive ? '#0AB000' : '#CF2318'} />
      </svg>
      <Text font={{ variation: FontVariation.SMALL }} color={isActive ? Color.GREY_800 : Color.RED_600}>
        {isActive ? 'Active' : 'Inactive'}
      </Text>
    </div>
  );
}

export default function FaultStudiosView({
  faultStudios,
  loading,
  searchTerm,
  setSearchTerm,
  refetch
}: FaultStudiosViewProps): React.ReactElement {
  const { getString } = useStrings();
  const history = useHistory();
  const paths = useRouteWithBaseUrl();

  // Modal state
  const { isOpen: isCreateModalOpen, open: openCreateModal, close: closeCreateModal } = useToggleOpen();

  // Delete dialog state
  const [deleteStudio, setDeleteStudio] = useState<{ id: string; name: string } | null>(null);

  useDocumentTitle(getString('faultStudios'));

  const breadcrumbs = [{ label: getString('faultStudios'), url: '' }];

  const handleCreateSuccess = (): void => {
    refetch();
  };

  const handleDeleteSuccess = (): void => {
    setDeleteStudio(null);
    refetch();
  };

  const HeaderToolbar = (
    <Layout.Horizontal spacing="medium" flex={{ alignItems: 'center' }}>
      <ExpandingSearchInput
        alwaysExpanded
        placeholder={getString('search')}
        onChange={text => setSearchTerm(text.trim())}
        defaultValue={searchTerm}
        width={250}
      />
      <Button variation={ButtonVariation.PRIMARY} text={getString('newFaultStudio')} onClick={openCreateModal} />
    </Layout.Horizontal>
  );

  return (
    <DefaultLayoutTemplate title={getString('faultStudios')} breadcrumbs={breadcrumbs} headerToolbar={HeaderToolbar}>
      <Container padding="medium">
        <Loader loading={loading}>
          {faultStudios.length === 0 ? (
            <NoFilteredData resetButton={<></>} />
          ) : (
            <Layout.Horizontal spacing="medium" className={css.cardsContainer}>
              {faultStudios.map(studio => (
                <Card
                  key={studio.id}
                  className={css.studioCard}
                  onClick={() => history.push(paths.toFaultStudio({ studioID: studio.id }))}
                >
                  <Layout.Vertical spacing="small">
                    <Layout.Horizontal flex={{ justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text font={{ variation: FontVariation.H5 }} color={Color.GREY_800}>
                        {studio.name}
                      </Text>
                      <Layout.Horizontal spacing="xsmall" flex={{ alignItems: 'center' }}>
                        <ActiveStatus isActive={studio.isActive} />
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
                                  icon="eye-open"
                                  text={getString('view')}
                                  onClick={() => history.push(paths.toFaultStudio({ studioID: studio.id }))}
                                  permission={PermissionGroup.VIEWER}
                                />
                                <RbacMenuItem
                                  icon="trash"
                                  text={getString('delete')}
                                  onClick={() => setDeleteStudio({ id: studio.id, name: studio.name })}
                                  permission={PermissionGroup.OWNER}
                                />
                              </Menu>
                            }
                          />
                        </div>
                      </Layout.Horizontal>
                    </Layout.Horizontal>
                    {studio.description && (
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500} lineClamp={2}>
                        {studio.description}
                      </Text>
                    )}
                    <Layout.Horizontal spacing="small">
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                        <Icon name="folder-open" size={12} /> {studio.totalFaults} faults
                      </Text>
                      <Text font={{ variation: FontVariation.SMALL }} color={Color.GREY_500}>
                        <Icon name="tick" size={12} /> {studio.enabledFaults} enabled
                      </Text>
                    </Layout.Horizontal>
                    <Text font={{ variation: FontVariation.TINY }} color={Color.GREY_400}>
                      Updated {getDetailedTime(parseInt(studio.updatedAt, 10))}
                    </Text>
                  </Layout.Vertical>
                </Card>
              ))}
            </Layout.Horizontal>
          )}
        </Loader>
      </Container>

      {/* Create Fault Studio Modal */}
      <CreateFaultStudioModalController
        isOpen={isCreateModalOpen}
        onClose={closeCreateModal}
        onSuccess={handleCreateSuccess}
      />

      {/* Delete Fault Studio Confirmation Dialog */}
      {deleteStudio && (
        <DeleteFaultStudioDialog
          isOpen={!!deleteStudio}
          onClose={() => setDeleteStudio(null)}
          studioId={deleteStudio.id}
          studioName={deleteStudio.name}
          onSuccess={handleDeleteSuccess}
        />
      )}
    </DefaultLayoutTemplate>
  );
}
