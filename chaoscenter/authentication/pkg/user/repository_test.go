package user

import (
	"testing"

	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/entities"
	"github.com/litmuschaos/litmus/chaoscenter/authentication/pkg/utils"

	"golang.org/x/crypto/bcrypt"
)

func TestCheckPasswordHash(t *testing.T) {
	password := "S3cret!ish"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}

	r := repository{}

	t.Run("matching password", func(t *testing.T) {
		if err := r.CheckPasswordHash(string(hash), password); err != nil {
			t.Errorf("expected match, got error %v", err)
		}
	})

	t.Run("wrong password returns ErrWrongPassword", func(t *testing.T) {
		err := r.CheckPasswordHash(string(hash), "not-the-password")
		if err != utils.ErrWrongPassword {
			t.Errorf("expected ErrWrongPassword, got %v", err)
		}
	})

	t.Run("invalid hash returns error", func(t *testing.T) {
		if err := r.CheckPasswordHash("not-a-bcrypt-hash", password); err != utils.ErrWrongPassword {
			t.Errorf("expected ErrWrongPassword, got %v", err)
		}
	})
}

func TestToDoc(t *testing.T) {
	t.Run("marshals struct to bson map", func(t *testing.T) {
		u := entities.User{ID: "u1", Username: "alice", Email: "a@x.com"}
		doc, err := toDoc(u)
		if err != nil {
			t.Fatalf("toDoc error: %v", err)
		}
		if doc == nil {
			t.Fatal("expected non-nil doc")
		}
		if (*doc)["username"] != "alice" {
			t.Errorf("expected username alice, got %v", (*doc)["username"])
		}
	})

	t.Run("unmarshalable input errors", func(t *testing.T) {
		// bson.Marshal requires a document type; a bare int fails to marshal.
		if _, err := toDoc(42); err == nil {
			t.Error("expected error marshaling non-document value")
		}
	})
}
