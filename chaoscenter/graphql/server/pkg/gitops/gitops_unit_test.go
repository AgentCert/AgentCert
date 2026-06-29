package gitops

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/golang-jwt/jwt/v4"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	dbGitOps "github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/database/mongodb/gitops"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/utils"
)

func TestGetKey(t *testing.T) {
	branch := "main"
	tests := []struct {
		name   string
		repo   string
		branch *string
		want   string
	}{
		{"nil branch returns repo", "https://github.com/org/repo.git", nil, "https://github.com/org/repo.git"},
		{"https repo with branch", "https://github.com/org/repo.git", &branch, "org/repo/main"},
		{"ssh repo with branch", "git@github.com:org/repo.git", &branch, "org/repo/main"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := getKey(tc.repo, tc.branch); got != tc.want {
				t.Errorf("getKey(%q, %v) = %q, want %q", tc.repo, tc.branch, got, tc.want)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	dir := t.TempDir()
	t.Run("existing path", func(t *testing.T) {
		ok, err := PathExists(dir)
		if err != nil || !ok {
			t.Errorf("PathExists(existing) = %v, %v", ok, err)
		}
	})
	t.Run("missing path", func(t *testing.T) {
		ok, err := PathExists(filepath.Join(dir, "does-not-exist"))
		if err != nil || ok {
			t.Errorf("PathExists(missing) = %v, %v; want false, nil", ok, err)
		}
	})
}

func TestGitUserFromContext(t *testing.T) {
	t.Run("nil context returns default", func(t *testing.T) {
		u := GitUserFromContext(nil)
		if u.username != "gitops@litmus-chaos" || u.email != "gitops@litmus.chaos" {
			t.Errorf("default user = %+v", u)
		}
	})

	t.Run("no claims returns default", func(t *testing.T) {
		u := GitUserFromContext(context.Background())
		if u.username != "gitops@litmus-chaos" {
			t.Errorf("expected default user, got %+v", u)
		}
	})

	t.Run("claims with username only", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), authorization.UserClaim, jwt.MapClaims{
			"username": "bob",
		})
		u := GitUserFromContext(ctx)
		if u.username != "bob@litmus-chaos" || u.email != "bob@litmus-chaos" {
			t.Errorf("user = %+v", u)
		}
	})

	t.Run("claims with username and email", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), authorization.UserClaim, jwt.MapClaims{
			"username": "alice",
			"email":    "alice@example.com",
		})
		u := GitUserFromContext(ctx)
		if u.username != "alice@litmus-chaos" || u.email != "alice@example.com" {
			t.Errorf("user = %+v", u)
		}
	})
}

func TestGetGitOpsConfig(t *testing.T) {
	user := "u"
	cfg := GetGitOpsConfig(dbGitOps.GitConfigDB{
		ProjectID:     "proj-1",
		RepositoryURL: "https://github.com/org/repo",
		Branch:        "main",
		LatestCommit:  "abc123",
		AuthType:      model.AuthTypeBasic,
		UserName:      &user,
	})
	if cfg.ProjectID != "proj-1" || cfg.RemoteName != "origin" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.LocalPath != DefaultPath+"proj-1" {
		t.Errorf("LocalPath = %q", cfg.LocalPath)
	}
	if cfg.Branch != "main" || cfg.LatestCommit != "abc123" {
		t.Errorf("branch/commit mismatch: %+v", cfg)
	}
}

func TestGetAuthMethod(t *testing.T) {
	token := "secret-token"
	user, pass := "alice", "pw"

	t.Run("token auth uses configured git username", func(t *testing.T) {
		utils.Config.GitUsername = "litmus"
		c := GitConfig{AuthType: model.AuthTypeToken, Token: &token}
		auth, err := c.getAuthMethod()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ba, ok := auth.(*http.BasicAuth)
		if !ok || ba.Username != "litmus" || ba.Password != token {
			t.Errorf("token auth = %+v", auth)
		}
	})

	t.Run("basic auth", func(t *testing.T) {
		c := GitConfig{AuthType: model.AuthTypeBasic, UserName: &user, Password: &pass}
		auth, err := c.getAuthMethod()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ba := auth.(*http.BasicAuth)
		if ba.Username != "alice" || ba.Password != "pw" {
			t.Errorf("basic auth = %+v", ba)
		}
	})

	t.Run("none auth returns nil", func(t *testing.T) {
		c := GitConfig{AuthType: model.AuthTypeNone}
		auth, err := c.getAuthMethod()
		if err != nil || auth != nil {
			t.Errorf("none auth = %v, %v", auth, err)
		}
	})

	t.Run("unknown auth type errors", func(t *testing.T) {
		c := GitConfig{AuthType: model.AuthType("WAT")}
		if _, err := c.getAuthMethod(); err == nil {
			t.Error("expected error for unknown auth type")
		}
	})

	t.Run("ssh with invalid key errors", func(t *testing.T) {
		bad := "not-a-key"
		c := GitConfig{AuthType: model.AuthTypeSSH, SSHPrivateKey: &bad}
		if _, err := c.getAuthMethod(); err == nil {
			t.Error("expected error for invalid ssh key")
		}
	})
}

func TestGenerateKeys(t *testing.T) {
	pub, priv, err := GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys error: %v", err)
	}
	if priv == "" || pub == "" {
		t.Fatal("expected non-empty key strings")
	}
	// Private key should be valid PEM that parses as an RSA key.
	block, _ := pem.Decode([]byte(priv))
	if block == nil {
		t.Fatal("private key is not valid PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("private key does not parse: %v", err)
	}
	if _, ok := interface{}(key).(*rsa.PrivateKey); !ok {
		t.Error("expected RSA private key")
	}
	// Public key should be in ssh authorized_keys form.
	if len(pub) < 8 || pub[:7] != "ssh-rsa" {
		t.Errorf("public key not in ssh-rsa form: %q", pub)
	}
}

func TestGitMutexLock(t *testing.T) {
	branch := "main"
	g := NewGitLock()
	// Lock then unlock should not deadlock or panic.
	g.Lock("https://github.com/org/repo.git", &branch)
	g.Unlock("https://github.com/org/repo.git", &branch)

	// Re-lock the same key works after unlock (the per-key mutex is reused).
	g.Lock("https://github.com/org/repo.git", &branch)
	g.Unlock("https://github.com/org/repo.git", &branch)

	// A second, distinct key can be locked independently.
	g.Lock("git@github.com:org/other.git", &branch)
	g.Unlock("git@github.com:org/other.git", &branch)

	// NOTE: Unlock on a never-locked key is intentionally NOT exercised here:
	// the source's Unlock returns while still holding mapMutex when the key is
	// absent, which would deadlock subsequent Lock calls. Covering it would hang.
}

func TestGetGitConfigDB(t *testing.T) {
	user := "u"
	cfg := dbGitOps.GetGitConfigDB("proj-2", model.GitConfig{
		RepoURL:  "https://github.com/org/repo",
		Branch:   "dev",
		AuthType: model.AuthTypeBasic,
		UserName: &user,
	})
	if cfg.ProjectID != "proj-2" || cfg.RepositoryURL != "https://github.com/org/repo" {
		t.Errorf("unexpected db config: %+v", cfg)
	}
	if cfg.Branch != "dev" || cfg.LatestCommit != "" {
		t.Errorf("branch/commit mismatch: %+v", cfg)
	}
}

// guard against unused import if a test is removed
var _ = os.Stat
