package validations

import (
	"errors"
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/entities"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/services"

	"go.mongodb.org/mongo-driver/bson"
)

// fakeApplicationService embeds the real interface so we only need to override
// the two methods RbacValidator actually calls. Any unused method will panic
// (nil interface) which keeps the fake honest about what it's exercising.
type fakeApplicationService struct {
	services.ApplicationService

	user        *entities.User
	getUserErr  error
	projects    []*entities.Project
	projectsErr error

	lastUID    string
	lastFilter bson.D
}

func (f *fakeApplicationService) GetUser(uid string) (*entities.User, error) {
	f.lastUID = uid
	return f.user, f.getUserErr
}

func (f *fakeApplicationService) GetProjects(query bson.D) ([]*entities.Project, error) {
	f.lastFilter = query
	return f.projects, f.projectsErr
}

func int64Ptr(v int64) *int64 { return &v }

func TestRbacValidator(t *testing.T) {
	roles := []string{string(entities.RoleOwner)}

	tests := []struct {
		name    string
		svc     *fakeApplicationService
		wantErr bool
	}{
		{
			name: "authorized owner",
			svc: &fakeApplicationService{
				user:     &entities.User{ID: "u1"},
				projects: []*entities.Project{{ID: "p1"}},
			},
			wantErr: false,
		},
		{
			name: "get user error propagates",
			svc: &fakeApplicationService{
				getUserErr: errors.New("db down"),
			},
			wantErr: true,
		},
		{
			name: "deactivated user rejected",
			svc: &fakeApplicationService{
				user: &entities.User{ID: "u1", DeactivatedAt: int64Ptr(123)},
			},
			wantErr: true,
		},
		{
			name: "get projects error propagates",
			svc: &fakeApplicationService{
				user:        &entities.User{ID: "u1"},
				projectsErr: errors.New("query failed"),
			},
			wantErr: true,
		},
		{
			name: "no matching project is unauthorized",
			svc: &fakeApplicationService{
				user:     &entities.User{ID: "u1"},
				projects: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RbacValidator("u1", "p1", roles, string(entities.AcceptedInvitation), tt.svc)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRbacValidator_PassesArgs(t *testing.T) {
	svc := &fakeApplicationService{
		user:     &entities.User{ID: "u1"},
		projects: []*entities.Project{{ID: "p1"}},
	}
	if err := RbacValidator("u1", "p1", []string{string(entities.RoleOwner)}, "Accepted", svc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastUID != "u1" {
		t.Errorf("GetUser called with %q, want u1", svc.lastUID)
	}
	if len(svc.lastFilter) == 0 {
		t.Error("expected a non-empty bson filter to be passed to GetProjects")
	}
}

func TestMutationRbacRules(t *testing.T) {
	// Owner-only mutations.
	ownerOnly := []string{"sendInvitation", "removeInvitation", "updateProjectName", "updateMemberRole", "deleteProject"}
	for _, op := range ownerOnly {
		roles, ok := MutationRbacRules[op]
		if !ok {
			t.Errorf("missing rbac rule for %q", op)
			continue
		}
		if len(roles) != 1 || roles[0] != string(entities.RoleOwner) {
			t.Errorf("%q should be owner-only, got %v", op, roles)
		}
	}

	// Mutations allowed for all three roles.
	allRoles := []string{"acceptInvitation", "declineInvitation", "leaveProject", "getProject"}
	for _, op := range allRoles {
		roles, ok := MutationRbacRules[op]
		if !ok {
			t.Errorf("missing rbac rule for %q", op)
			continue
		}
		if len(roles) != 3 {
			t.Errorf("%q should allow 3 roles, got %v", op, roles)
		}
	}
}
