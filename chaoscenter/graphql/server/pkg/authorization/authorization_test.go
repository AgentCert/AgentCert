package authorization

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

const testSalt = "test-secret-salt"

func signHS256(t *testing.T, claims jwt.MapClaims, salt string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(salt))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestUserValidateJWT(t *testing.T) {
	validToken := signHS256(t, jwt.MapClaims{
		"username": "admin",
		"exp":      time.Now().Add(time.Hour).Unix(),
	}, testSalt)

	wrongSaltToken := signHS256(t, jwt.MapClaims{
		"username": "admin",
		"exp":      time.Now().Add(time.Hour).Unix(),
	}, "other-salt")

	expiredToken := signHS256(t, jwt.MapClaims{
		"username": "admin",
		"exp":      time.Now().Add(-time.Hour).Unix(),
	}, testSalt)

	// Token signed with RS-like non-HMAC alg header is rejected by the keyfunc.
	rsToken := func() string {
		tok := jwt.New(jwt.SigningMethodNone)
		tok.Claims = jwt.MapClaims{"username": "admin"}
		s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		return s
	}()

	tests := []struct {
		name      string
		token     string
		salt      string
		wantErr   bool
		wantUser  string
	}{
		{"valid", validToken, testSalt, false, "admin"},
		{"wrong salt", wrongSaltToken, testSalt, true, ""},
		{"expired", expiredToken, testSalt, true, ""},
		{"non-hmac alg rejected", rsToken, testSalt, true, ""},
		{"garbage token", "not.a.jwt", testSalt, true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := UserValidateJWT(tc.token, tc.salt)
			if (err != nil) != tc.wantErr {
				t.Fatalf("UserValidateJWT() err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if claims == nil {
					t.Fatal("expected claims, got nil")
				}
				if claims["username"] != tc.wantUser {
					t.Errorf("username = %v, want %v", claims["username"], tc.wantUser)
				}
			}
		})
	}
}

func TestMutationRbacRules(t *testing.T) {
	owner := string(model.MemberRoleOwner)
	executor := string(model.MemberRoleExecutor)
	viewer := string(model.MemberRoleViewer)

	contains := func(s []string, v string) bool {
		for _, x := range s {
			if x == v {
				return true
			}
		}
		return false
	}

	// Owner-only mutations.
	for _, q := range []RoleQuery{CreateChaosExperiment, DeleteChaosExperiment, AddChaosHub, EnableGitOps, AddProbe} {
		roles, ok := MutationRbacRules[q]
		if !ok {
			t.Errorf("missing rule for %s", q)
			continue
		}
		if !contains(roles, owner) || contains(roles, viewer) {
			t.Errorf("%s should be owner-only, got %v", q, roles)
		}
	}

	// Read queries available to all three roles.
	for _, q := range []RoleQuery{ListExperiment, ListWorkflowRuns, GetEnvironment, ListProbes} {
		roles := MutationRbacRules[q]
		if !contains(roles, owner) || !contains(roles, executor) || !contains(roles, viewer) {
			t.Errorf("%s should allow all roles, got %v", q, roles)
		}
	}

	// Executor-permitted action that excludes viewer.
	if roles := MutationRbacRules[StopChaosExperiment]; !contains(roles, executor) || contains(roles, viewer) {
		t.Errorf("StopChaosExperiment roles = %v, want owner+executor", roles)
	}
}
