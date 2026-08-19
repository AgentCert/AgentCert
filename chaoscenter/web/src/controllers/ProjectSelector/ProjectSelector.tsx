import React from 'react';
import { useParams } from 'react-router-dom';
import ProjectSelectorView from '@views/ProjectSelector';
import { useGetProjectQuery, useGetProjectRoleQuery } from '@api/auth';
import { useAppStore } from '@context';
import { setUserDetails } from '@utils';

export default function ProjectSelectorController(): React.ReactElement {
  const { projectID } = useParams<{ projectID: string }>();
  const { updateAppStore } = useAppStore();

  const { data: projectData } = useGetProjectQuery(
    {
      project_id: projectID
    },
    {
      enabled: !!projectID,
      onSuccess: data => {
        updateAppStore({
          projectName: data.data?.name
        });
      }
    }
  );

  // Re-validate the caller's role in this project on every mount (i.e. every reload of a
  // project-scoped route), not just at login time. projectRole is otherwise only ever written
  // once, at login — so a role change since then, or a stale/corrupt localStorage value, would
  // otherwise persist indefinitely and silently disable RBAC-gated actions (e.g. Save in Chaos
  // Studio) with no way to recover short of an explicit logout/login.
  useGetProjectRoleQuery(
    { project_id: projectID },
    {
      enabled: !!projectID,
      onSuccess: data => {
        if (data.role) {
          updateAppStore({ projectRole: data.role });
          setUserDetails({ projectRole: data.role });
        }
      }
    }
  );

  return <ProjectSelectorView currentProjectDetails={projectData?.data} />;
}
