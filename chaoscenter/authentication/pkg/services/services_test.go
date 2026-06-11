package services

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/authConfig"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/entities"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---- fake repositories ----
// Each stub implements only the repository interface it stands in for, so a
// signature drift in the real interface will break compilation here.

type stubUserRepo struct {
	loginUser   *entities.User
	loginErr    error
	getUser     *entities.User
	getUserErr  error
	users       *[]entities.User
	usersErr    error
	checkHash   error
	createUser  *entities.User
	createErr   error
	updateErr   error
	isAdminErr  error
	invited     *[]entities.User
	invitedErr  error
	stateErr    error
	findByName  *entities.User
	findNameErr error
}

func (s *stubUserRepo) LoginUser(u *entities.User) (*entities.User, error) { return s.loginUser, s.loginErr }
func (s *stubUserRepo) GetUser(uid string) (*entities.User, error)         { return s.getUser, s.getUserErr }
func (s *stubUserRepo) GetUsers() (*[]entities.User, error)                { return s.users, s.usersErr }
func (s *stubUserRepo) FindUsersByUID(uid []string) (*[]entities.User, error) {
	return s.users, s.usersErr
}
func (s *stubUserRepo) FindUserByUsername(username string) (*entities.User, error) {
	return s.findByName, s.findNameErr
}
func (s *stubUserRepo) CheckPasswordHash(hash, password string) error { return s.checkHash }
func (s *stubUserRepo) UpdatePassword(p *entities.UserPassword, isAdminBeingReset bool) error {
	return s.updateErr
}
func (s *stubUserRepo) CreateUser(u *entities.User) (*entities.User, error) {
	return s.createUser, s.createErr
}
func (s *stubUserRepo) UpdateUser(u *entities.UserDetails) error          { return s.updateErr }
func (s *stubUserRepo) UpdateUserByQuery(f bson.D, q bson.D) error        { return s.updateErr }
func (s *stubUserRepo) IsAdministrator(u *entities.User) error           { return s.isAdminErr }
func (s *stubUserRepo) UpdateUserState(ctx context.Context, username string, isDeactivate bool, deactivateTime int64) error {
	return s.stateErr
}
func (s *stubUserRepo) InviteUsers(invitedUsers []string) (*[]entities.User, error) {
	return s.invited, s.invitedErr
}

type stubProjectRepo struct {
	project    *entities.Project
	projectErr error
	projects   []*entities.Project
	listErr    error
	createErr  error
}

func (s *stubProjectRepo) GetProjectByProjectID(projectID string) (*entities.Project, error) {
	return s.project, s.projectErr
}
func (s *stubProjectRepo) GetProjects(query bson.D) ([]*entities.Project, error) {
	return s.projects, s.listErr
}
func (s *stubProjectRepo) GetProjectsByUserID(request *entities.ListProjectRequest) (*entities.ListProjectResponse, error) {
	return nil, s.listErr
}
func (s *stubProjectRepo) GetProjectStats() ([]*entities.ProjectStats, error) { return nil, s.listErr }
func (s *stubProjectRepo) CreateProject(project *entities.Project) error      { return s.createErr }
func (s *stubProjectRepo) AddMember(projectID string, member *entities.Member) error {
	return s.createErr
}
func (s *stubProjectRepo) RemoveInvitation(projectID string, userID string, invitation entities.Invitation) error {
	return s.createErr
}
func (s *stubProjectRepo) UpdateInvite(projectID string, userID string, invitation entities.Invitation, role *entities.MemberRole) error {
	return s.createErr
}
func (s *stubProjectRepo) UpdateProjectName(projectID string, projectName string) error {
	return s.createErr
}
func (s *stubProjectRepo) UpdateMemberRole(projectID string, userID string, role *entities.MemberRole) error {
	return s.createErr
}
func (s *stubProjectRepo) GetAggregateProjects(pipeline mongo.Pipeline, opts *options.AggregateOptions) (*mongo.Cursor, error) {
	return nil, s.listErr
}
func (s *stubProjectRepo) UpdateProjectState(ctx context.Context, userID string, deactivateTime int64, isDeactivate bool) error {
	return s.createErr
}
func (s *stubProjectRepo) GetOwnerProjects(ctx context.Context, userID string) ([]*entities.Project, error) {
	return s.projects, s.listErr
}
func (s *stubProjectRepo) GetProjectRole(projectID string, userID string) (*entities.MemberRole, error) {
	return nil, s.listErr
}
func (s *stubProjectRepo) GetProjectMembers(projectID string, state string) ([]*entities.Member, error) {
	return nil, s.listErr
}
func (s *stubProjectRepo) GetProjectOwners(projectID string) ([]*entities.Member, error) {
	return nil, s.listErr
}
func (s *stubProjectRepo) ListInvitations(userID string, invitationState entities.Invitation) ([]*entities.Project, error) {
	return s.projects, s.listErr
}
func (s *stubProjectRepo) DeleteProject(projectID string) error { return s.createErr }

type stubMiscRepo struct {
	collections []string
	colErr      error
	databases   []string
	dbErr       error
}

func (s *stubMiscRepo) ListCollection() ([]string, error) { return s.collections, s.colErr }
func (s *stubMiscRepo) ListDataBase() ([]string, error)   { return s.databases, s.dbErr }

type stubRevokedTokenRepo struct {
	revokeErr error
	revoked   bool
	lastToken *entities.RevokedToken
}

func (s *stubRevokedTokenRepo) RevokeToken(token *entities.RevokedToken) error {
	s.lastToken = token
	return s.revokeErr
}
func (s *stubRevokedTokenRepo) IsTokenRevoked(encodedToken string) bool { return s.revoked }

type stubApiTokenRepo struct {
	createErr error
	tokens    []entities.ApiToken
	getErr    error
	deleteErr error
	created   *entities.ApiToken
}

func (s *stubApiTokenRepo) CreateApiToken(apiToken *entities.ApiToken) error {
	s.created = apiToken
	return s.createErr
}
func (s *stubApiTokenRepo) GetApiTokensByUserID(userID string) ([]entities.ApiToken, error) {
	return s.tokens, s.getErr
}
func (s *stubApiTokenRepo) DeleteApiToken(token string) error { return s.deleteErr }

type stubAuthConfigRepo struct {
	config    *authConfig.AuthConfig
	getErr    error
	createErr error
	updateErr error
}

func (s *stubAuthConfigRepo) CreateConfig(config authConfig.AuthConfig) error { return s.createErr }
func (s *stubAuthConfigRepo) GetConfig(key string) (*authConfig.AuthConfig, error) {
	return s.config, s.getErr
}
func (s *stubAuthConfigRepo) UpdateConfig(ctx context.Context, key string, value interface{}) error {
	return s.updateErr
}

// build constructs an ApplicationService backed by the provided stubs.
func build(u *stubUserRepo, p *stubProjectRepo, m *stubMiscRepo, rt *stubRevokedTokenRepo, at *stubApiTokenRepo, ac *stubAuthConfigRepo) ApplicationService {
	if u == nil {
		u = &stubUserRepo{}
	}
	if p == nil {
		p = &stubProjectRepo{}
	}
	if m == nil {
		m = &stubMiscRepo{}
	}
	if rt == nil {
		rt = &stubRevokedTokenRepo{}
	}
	if at == nil {
		at = &stubApiTokenRepo{}
	}
	if ac == nil {
		ac = &stubAuthConfigRepo{}
	}
	return NewService(u, p, m, rt, at, ac, nil)
}

// ---- session service (JWT) tests ----

func TestGetSignedJWT(t *testing.T) {
	svc := build(nil, nil, nil, nil, nil, nil)
	user := &entities.User{ID: "u1", Role: entities.RoleAdmin, Username: "alice"}

	tokenStr, err := svc.GetSignedJWT(user, "my-secret")
	if err != nil {
		t.Fatalf("GetSignedJWT error: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	parsed, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte("my-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token did not validate: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["uid"] != "u1" || claims["username"] != "alice" {
		t.Errorf("unexpected claims: %v", claims)
	}
}

func TestValidateToken(t *testing.T) {
	secret := "salt-secret"
	ac := &stubAuthConfigRepo{config: &authConfig.AuthConfig{Key: "salt", Value: secret}}

	t.Run("valid token", func(t *testing.T) {
		svc := build(nil, nil, nil, &stubRevokedTokenRepo{revoked: false}, nil, ac)
		user := &entities.User{ID: "u1", Role: entities.RoleUser, Username: "bob"}
		tokenStr, _ := svc.GetSignedJWT(user, secret)

		tok, err := svc.ValidateToken(tokenStr)
		if err != nil {
			t.Fatalf("ValidateToken error: %v", err)
		}
		if !tok.Valid {
			t.Error("expected token to be valid")
		}
	})

	t.Run("revoked token rejected", func(t *testing.T) {
		svc := build(nil, nil, nil, &stubRevokedTokenRepo{revoked: true}, nil, ac)
		user := &entities.User{ID: "u1"}
		tokenStr, _ := svc.GetSignedJWT(user, secret)

		tok, err := svc.ValidateToken(tokenStr)
		if err == nil {
			t.Fatal("expected error for revoked token")
		}
		if tok.Valid {
			t.Error("expected revoked token to be invalid")
		}
	})

	t.Run("config fetch failure", func(t *testing.T) {
		failAc := &stubAuthConfigRepo{getErr: errors.New("no salt")}
		svc := build(nil, nil, nil, nil, nil, failAc)
		user := &entities.User{ID: "u1"}
		tokenStr, _ := svc.GetSignedJWT(user, secret)

		if _, err := svc.ValidateToken(tokenStr); err == nil {
			t.Fatal("expected error when config cannot be fetched")
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		svc := build(nil, nil, nil, nil, nil, ac)
		if _, err := svc.ValidateToken("garbage.token.value"); err == nil {
			t.Fatal("expected error for garbage token")
		}
	})
}

func TestRevokeToken(t *testing.T) {
	secret := "salt-secret"
	ac := &stubAuthConfigRepo{config: &authConfig.AuthConfig{Key: "salt", Value: secret}}

	t.Run("revokes valid token", func(t *testing.T) {
		rt := &stubRevokedTokenRepo{}
		svc := build(nil, nil, nil, rt, nil, ac)
		user := &entities.User{ID: "u1"}
		tokenStr, _ := svc.GetSignedJWT(user, secret)

		if err := svc.RevokeToken(tokenStr); err != nil {
			t.Fatalf("RevokeToken error: %v", err)
		}
		if rt.lastToken == nil || rt.lastToken.Token != tokenStr {
			t.Error("expected token to be passed to repository")
		}
		if rt.lastToken.ExpiresAt == 0 {
			t.Error("expected ExpiresAt to be populated from claims")
		}
	})

	t.Run("invalid token cannot be revoked", func(t *testing.T) {
		svc := build(nil, nil, nil, &stubRevokedTokenRepo{}, nil, ac)
		if err := svc.RevokeToken("not-a-token"); err == nil {
			t.Fatal("expected error for invalid token")
		}
	})

	t.Run("repository error propagates", func(t *testing.T) {
		rt := &stubRevokedTokenRepo{revokeErr: errors.New("write failed")}
		svc := build(nil, nil, nil, rt, nil, ac)
		user := &entities.User{ID: "u1"}
		tokenStr, _ := svc.GetSignedJWT(user, secret)
		if err := svc.RevokeToken(tokenStr); err == nil {
			t.Fatal("expected repository error to propagate")
		}
	})
}

func TestCreateApiToken(t *testing.T) {
	secret := "salt-secret"
	ac := &stubAuthConfigRepo{config: &authConfig.AuthConfig{Key: "salt", Value: secret}}

	t.Run("creates and persists token", func(t *testing.T) {
		at := &stubApiTokenRepo{}
		svc := build(nil, nil, nil, nil, at, ac)
		user := &entities.User{ID: "u1", Username: "alice", Role: entities.RoleUser}

		tokenStr, err := svc.CreateApiToken(user, entities.ApiTokenInput{Name: "ci", DaysUntilExpiration: 30})
		if err != nil {
			t.Fatalf("CreateApiToken error: %v", err)
		}
		if tokenStr == "" {
			t.Fatal("expected non-empty token")
		}
		if at.created == nil || at.created.Name != "ci" || at.created.UserID != "u1" {
			t.Errorf("unexpected persisted token: %+v", at.created)
		}
		if at.created.ExpiresAt == 0 {
			t.Error("expected ExpiresAt populated")
		}
	})

	t.Run("config fetch failure", func(t *testing.T) {
		failAc := &stubAuthConfigRepo{getErr: errors.New("no salt")}
		svc := build(nil, nil, nil, nil, &stubApiTokenRepo{}, failAc)
		user := &entities.User{ID: "u1"}
		if _, err := svc.CreateApiToken(user, entities.ApiTokenInput{DaysUntilExpiration: 1}); err == nil {
			t.Fatal("expected error when config cannot be fetched")
		}
	})

	t.Run("repository error propagates", func(t *testing.T) {
		at := &stubApiTokenRepo{createErr: errors.New("insert failed")}
		svc := build(nil, nil, nil, nil, at, ac)
		user := &entities.User{ID: "u1"}
		if _, err := svc.CreateApiToken(user, entities.ApiTokenInput{DaysUntilExpiration: 1}); err == nil {
			t.Fatal("expected repository error to propagate")
		}
	})
}

func TestApiTokenPassThrough(t *testing.T) {
	tokens := []entities.ApiToken{{UserID: "u1", Name: "ci"}}
	at := &stubApiTokenRepo{tokens: tokens}
	svc := build(nil, nil, nil, nil, at, nil)

	got, err := svc.GetApiTokensByUserID("u1")
	if err != nil || len(got) != 1 || got[0].Name != "ci" {
		t.Fatalf("GetApiTokensByUserID = %v, %v", got, err)
	}

	if err := svc.DeleteApiToken("tok"); err != nil {
		t.Errorf("DeleteApiToken error: %v", err)
	}

	atErr := &stubApiTokenRepo{deleteErr: errors.New("nope")}
	svcErr := build(nil, nil, nil, nil, atErr, nil)
	if err := svcErr.DeleteApiToken("tok"); err == nil {
		t.Error("expected delete error to propagate")
	}
}

// ---- auth config service ----

func TestAuthConfigService(t *testing.T) {
	t.Run("get config success", func(t *testing.T) {
		ac := &stubAuthConfigRepo{config: &authConfig.AuthConfig{Key: "salt", Value: "v"}}
		svc := build(nil, nil, nil, nil, nil, ac)
		got, err := svc.GetConfig("salt")
		if err != nil || got.Value != "v" {
			t.Fatalf("GetConfig = %+v, %v", got, err)
		}
	})

	t.Run("create config error", func(t *testing.T) {
		ac := &stubAuthConfigRepo{createErr: errors.New("dup")}
		svc := build(nil, nil, nil, nil, nil, ac)
		if err := svc.CreateConfig(authConfig.AuthConfig{Key: "k"}); err == nil {
			t.Error("expected create error to propagate")
		}
	})

	t.Run("update config", func(t *testing.T) {
		svc := build(nil, nil, nil, nil, nil, &stubAuthConfigRepo{})
		if err := svc.UpdateConfig(context.Background(), "salt", "newval"); err != nil {
			t.Errorf("UpdateConfig error: %v", err)
		}
		acErr := &stubAuthConfigRepo{updateErr: errors.New("boom")}
		svcErr := build(nil, nil, nil, nil, nil, acErr)
		if err := svcErr.UpdateConfig(context.Background(), "salt", "newval"); err == nil {
			t.Error("expected update error to propagate")
		}
	})
}

// ---- misc service ----

func TestMiscService(t *testing.T) {
	m := &stubMiscRepo{collections: []string{"users", "project"}, databases: []string{"auth"}}
	svc := build(nil, nil, m, nil, nil, nil)

	cols, err := svc.ListCollection()
	if err != nil || len(cols) != 2 {
		t.Fatalf("ListCollection = %v, %v", cols, err)
	}
	dbs, err := svc.ListDataBase()
	if err != nil || len(dbs) != 1 {
		t.Fatalf("ListDataBase = %v, %v", dbs, err)
	}

	mErr := &stubMiscRepo{colErr: errors.New("x"), dbErr: errors.New("y")}
	svcErr := build(nil, nil, mErr, nil, nil, nil)
	if _, err := svcErr.ListCollection(); err == nil {
		t.Error("expected collection error to propagate")
	}
	if _, err := svcErr.ListDataBase(); err == nil {
		t.Error("expected database error to propagate")
	}
}

// ---- user service pass-through ----

func TestUserServicePassThrough(t *testing.T) {
	user := &entities.User{ID: "u1", Username: "alice"}
	u := &stubUserRepo{
		getUser:    user,
		findByName: user,
		loginUser:  user,
		createUser: user,
		users:      &[]entities.User{*user},
	}
	svc := build(u, nil, nil, nil, nil, nil)

	if got, _ := svc.GetUser("u1"); got == nil || got.ID != "u1" {
		t.Error("GetUser pass-through failed")
	}
	if got, _ := svc.FindUserByUsername("alice"); got == nil || got.Username != "alice" {
		t.Error("FindUserByUsername pass-through failed")
	}
	if got, _ := svc.LoginUser(user); got == nil {
		t.Error("LoginUser pass-through failed")
	}
	if got, _ := svc.CreateUser(user); got == nil {
		t.Error("CreateUser pass-through failed")
	}
	if got, _ := svc.GetUsers(); got == nil || len(*got) != 1 {
		t.Error("GetUsers pass-through failed")
	}
	if got, _ := svc.FindUsersByUID([]string{"u1"}); got == nil {
		t.Error("FindUsersByUID pass-through failed")
	}
	if err := svc.CheckPasswordHash("h", "p"); err != nil {
		t.Error("CheckPasswordHash pass-through failed")
	}
	if err := svc.IsAdministrator(user); err != nil {
		t.Error("IsAdministrator pass-through failed")
	}
	if err := svc.UpdateUser(&entities.UserDetails{ID: "u1"}); err != nil {
		t.Error("UpdateUser pass-through failed")
	}
	if err := svc.UpdateUserByQuery(bson.D{}, bson.D{}); err != nil {
		t.Error("UpdateUserByQuery pass-through failed")
	}
	if err := svc.UpdatePassword(&entities.UserPassword{}, false); err != nil {
		t.Error("UpdatePassword pass-through failed")
	}
	if err := svc.UpdateUserState(context.Background(), "alice", true, 1); err != nil {
		t.Error("UpdateUserState pass-through failed")
	}
}

func TestUserServiceErrorPropagation(t *testing.T) {
	u := &stubUserRepo{getUserErr: errors.New("not found")}
	svc := build(u, nil, nil, nil, nil, nil)
	if _, err := svc.GetUser("x"); err == nil {
		t.Error("expected GetUser error to propagate")
	}
}

// ---- project service pass-through ----

func TestProjectServicePassThrough(t *testing.T) {
	proj := &entities.Project{ID: "p1", Name: "Proj"}
	p := &stubProjectRepo{
		project:  proj,
		projects: []*entities.Project{proj},
	}
	svc := build(nil, p, nil, nil, nil, nil)

	if got, _ := svc.GetProjectByProjectID("p1"); got == nil || got.ID != "p1" {
		t.Error("GetProjectByProjectID pass-through failed")
	}
	if got, _ := svc.GetProjects(bson.D{}); len(got) != 1 {
		t.Error("GetProjects pass-through failed")
	}
	if got, _ := svc.ListInvitations("u1", entities.PendingInvitation); len(got) != 1 {
		t.Error("ListInvitations pass-through failed")
	}
	if got, _ := svc.GetOwnerProjectIDs(context.Background(), "u1"); len(got) != 1 {
		t.Error("GetOwnerProjectIDs pass-through failed")
	}
	if err := svc.CreateProject(proj); err != nil {
		t.Error("CreateProject pass-through failed")
	}
	if err := svc.AddMember("p1", &entities.Member{UserID: "u2"}); err != nil {
		t.Error("AddMember pass-through failed")
	}
	if err := svc.UpdateProjectName("p1", "new"); err != nil {
		t.Error("UpdateProjectName pass-through failed")
	}
	if err := svc.DeleteProject("p1"); err != nil {
		t.Error("DeleteProject pass-through failed")
	}
	role := entities.RoleViewer
	if err := svc.UpdateMemberRole("p1", "u2", &role); err != nil {
		t.Error("UpdateMemberRole pass-through failed")
	}
	if err := svc.RemoveInvitation("p1", "u2", entities.PendingInvitation); err != nil {
		t.Error("RemoveInvitation pass-through failed")
	}
	if err := svc.UpdateInvite("p1", "u2", entities.AcceptedInvitation, &role); err != nil {
		t.Error("UpdateInvite pass-through failed")
	}
	if err := svc.UpdateProjectState(context.Background(), "u1", 0, false); err != nil {
		t.Error("UpdateProjectState pass-through failed")
	}

	// remaining read pass-throughs (stubs return nil/empty without error)
	if _, err := svc.GetProjectsByUserID(&entities.ListProjectRequest{UserID: "u1"}); err != nil {
		t.Error("GetProjectsByUserID pass-through failed")
	}
	if _, err := svc.GetProjectStats(); err != nil {
		t.Error("GetProjectStats pass-through failed")
	}
	if _, err := svc.GetAggregateProjects(mongo.Pipeline{}, nil); err != nil {
		t.Error("GetAggregateProjects pass-through failed")
	}
	if _, err := svc.GetProjectRole("p1", "u1"); err != nil {
		t.Error("GetProjectRole pass-through failed")
	}
	if _, err := svc.GetProjectMembers("p1", "active"); err != nil {
		t.Error("GetProjectMembers pass-through failed")
	}
	if _, err := svc.GetProjectOwners("p1"); err != nil {
		t.Error("GetProjectOwners pass-through failed")
	}
}

func TestInviteUsersPassThrough(t *testing.T) {
	invited := []entities.User{{ID: "u3"}}
	u := &stubUserRepo{invited: &invited}
	svc := build(u, nil, nil, nil, nil, nil)
	got, err := svc.InviteUsers([]string{"u3"})
	if err != nil || got == nil || len(*got) != 1 {
		t.Fatalf("InviteUsers = %v, %v", got, err)
	}
}

// Ensure the constructed service satisfies the full interface and our stubs
// match the repository signatures, guarding against drift.
func TestNewServiceSatisfiesInterface(t *testing.T) {
	var _ ApplicationService = build(nil, nil, nil, nil, nil, nil)
	// avoid unused warnings for the error sentinels imported transitively
	_ = utils.ErrServerError
}
