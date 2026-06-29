package entities

import "testing"

func TestSanitizedUser(t *testing.T) {
	u := &User{
		ID:       "uid-1",
		Username: "alice",
		Password: "super-secret",
		Salt:     "salt",
	}
	got := u.SanitizedUser()
	if got.Password != "" {
		t.Errorf("expected password cleared, got %q", got.Password)
	}
	// Non-sensitive fields preserved.
	if got.Username != "alice" || got.ID != "uid-1" {
		t.Errorf("unexpected sanitized fields: %+v", got)
	}
	// Original is mutated in place (documenting real behavior).
	if u.Password != "" {
		t.Error("expected original password to be cleared in place")
	}
}

func TestIsEmailValid(t *testing.T) {
	u := &User{}
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"first.last@sub.domain.org", true},
		{"plainaddress", false},
		{"", false},
		{"@no-local.com", false},
		{"missing-domain@", false},
		{"spaces in@email.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := u.IsEmailValid(tt.email); got != tt.want {
				t.Errorf("IsEmailValid(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestProjectGetMemberOutput(t *testing.T) {
	role := RoleOwner
	m := &Member{
		UserID:     "u1",
		Username:   "bob",
		Email:      "bob@example.com",
		Name:       "Bob",
		Role:       role,
		Invitation: AcceptedInvitation,
		JoinedAt:   1234,
	}
	out := m.GetMemberOutput()
	if out.UserID != "u1" {
		t.Errorf("UserID = %q, want u1", out.UserID)
	}
	if out.Role != role {
		t.Errorf("Role = %q, want %q", out.Role, role)
	}
	if out.Invitation != AcceptedInvitation {
		t.Errorf("Invitation = %q, want %q", out.Invitation, AcceptedInvitation)
	}
	if out.JoinedAt != 1234 {
		t.Errorf("JoinedAt = %d, want 1234", out.JoinedAt)
	}
	// GetMemberOutput intentionally drops PII like email/username/name.
	if out.Email != "" || out.Username != "" || out.Name != "" {
		t.Errorf("expected email/username/name dropped, got %+v", out)
	}
}

func TestProjectGetMemberOutput_Slice(t *testing.T) {
	p := &Project{
		Members: []*Member{
			{UserID: "u1", Role: RoleOwner},
			{UserID: "u2", Role: RoleViewer},
		},
	}
	out := p.GetMemberOutput()
	if len(out) != 2 {
		t.Fatalf("expected 2 members, got %d", len(out))
	}
	if out[0].UserID != "u1" || out[1].UserID != "u2" {
		t.Errorf("unexpected member ids: %+v", out)
	}

	t.Run("empty project yields nil", func(t *testing.T) {
		empty := &Project{}
		if got := empty.GetMemberOutput(); got != nil {
			t.Errorf("expected nil for no members, got %+v", got)
		}
	})
}

func TestProjectGetProjectOutput(t *testing.T) {
	state := "active"
	updatedBy := UserDetailResponse{UserID: "owner", Username: "owner-name", Email: "o@x.com"}
	p := &Project{
		ID:    "proj-1",
		Name:  "My Project",
		State: &state,
		Members: []*Member{
			{UserID: "u1", Role: RoleOwner},
		},
		Audit: Audit{
			IsRemoved: false,
			CreatedAt: 100,
			UpdatedAt: 200,
			UpdatedBy: updatedBy,
		},
	}
	out := p.GetProjectOutput()
	if out.ID != "proj-1" || out.Name != "My Project" {
		t.Errorf("unexpected id/name: %+v", out)
	}
	if out.State == nil || *out.State != "active" {
		t.Errorf("unexpected state: %v", out.State)
	}
	if len(out.Members) != 1 || out.Members[0].UserID != "u1" {
		t.Errorf("unexpected members: %+v", out.Members)
	}
	if out.CreatedAt != 100 || out.UpdatedAt != 200 {
		t.Errorf("unexpected audit timestamps: %+v", out.Audit)
	}
	// GetProjectOutput sets CreatedBy from UpdatedBy (documenting current behavior).
	if out.CreatedBy.UserID != "owner" {
		t.Errorf("CreatedBy = %+v, want derived from UpdatedBy", out.CreatedBy)
	}
}

func TestRoleAndInvitationConstants(t *testing.T) {
	if RoleOwner != "Owner" || RoleExecutor != "Executor" || RoleViewer != "Viewer" {
		t.Error("MemberRole constants changed unexpectedly")
	}
	if PendingInvitation != "Pending" || AcceptedInvitation != "Accepted" ||
		DeclinedInvitation != "Declined" || ExitedProject != "Exited" {
		t.Error("Invitation constants changed unexpectedly")
	}
	if RoleAdmin != "admin" || RoleUser != "user" {
		t.Error("user Role constants changed unexpectedly")
	}
}
