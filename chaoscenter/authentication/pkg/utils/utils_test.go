package utils

import (
	"os"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trims leading and trailing spaces", "  hello  ", "hello"},
		{"trims tabs and newlines", "\t\nhello\n\t", "hello"},
		{"no whitespace", "hello", "hello"},
		{"empty string", "", ""},
		{"only whitespace", "   ", ""},
		{"internal spaces preserved", "  a b  ", "a b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeString(tt.input); got != tt.want {
				t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateStrictPassword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errSub  string
	}{
		{"valid password", "Abcd123!", false, ""},
		{"valid max length", "Abcdefgh1234567!", false, ""},
		{"too short", "Ab1!", true, "less than 8"},
		{"too long", "Abcdefgh123456789!", true, "more than 16"},
		{"no digit", "Abcdefg!", true, "digits"},
		{"no lowercase", "ABCD123!", true, "lowercase"},
		{"no uppercase", "abcd123!", true, "uppercase"},
		{"no special char", "Abcd1234", true, "special"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrictPassword(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if tt.wantErr && tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestValidateStrictUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"valid simple", "abc", false},
		{"valid with digits", "abc123", false},
		{"valid with underscore and hyphen", "a_b-c", false},
		{"valid max length", "abcdefghijklmnop", false},
		{"too short", "ab", true},
		{"starts with digit", "1abc", true},
		{"starts with underscore", "_abc", true},
		{"contains space", "ab c", true},
		{"contains special char", "ab$c", true},
		{"too long", "abcdefghijklmnopq", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrictUsername(tt.username)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.username)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.username, err)
			}
		})
	}
}

func TestRandomString(t *testing.T) {
	t.Run("positive length produces output", func(t *testing.T) {
		s, err := RandomString(16)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("distinct outputs", func(t *testing.T) {
		a, _ := RandomString(16)
		b, _ := RandomString(16)
		if a == b {
			t.Error("expected two random strings to differ")
		}
	})

	t.Run("zero length errors", func(t *testing.T) {
		if _, err := RandomString(0); err == nil {
			t.Error("expected error for zero length")
		}
	})

	t.Run("negative length errors", func(t *testing.T) {
		if _, err := RandomString(-5); err == nil {
			t.Error("expected error for negative length")
		}
	})
}

func TestGenerateAndValidateOAuthJWT(t *testing.T) {
	OAuthJwtSecret = "test-oauth-secret"
	OAuthJWTExpDuration = 5

	token, err := GenerateOAuthJWT()
	if err != nil {
		t.Fatalf("GenerateOAuthJWT returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	valid, err := ValidateOAuthJWT(token)
	if err != nil {
		t.Fatalf("ValidateOAuthJWT returned error: %v", err)
	}
	if !valid {
		t.Error("expected token to be valid")
	}
}

func TestValidateOAuthJWT_Invalid(t *testing.T) {
	OAuthJwtSecret = "test-oauth-secret"

	t.Run("garbage token", func(t *testing.T) {
		valid, err := ValidateOAuthJWT("not-a-real-token")
		if err == nil {
			t.Error("expected error for garbage token")
		}
		if valid {
			t.Error("expected token to be invalid")
		}
	})

	t.Run("wrong signing method", func(t *testing.T) {
		// Token signed with a "none" alg should be rejected by the HMAC check.
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"exp": 9999999999})
		s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		valid, err := ValidateOAuthJWT(s)
		if err == nil {
			t.Error("expected error for non-HMAC token")
		}
		if valid {
			t.Error("expected token to be invalid")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		OAuthJwtSecret = "secret-a"
		token, _ := GenerateOAuthJWT()
		OAuthJwtSecret = "secret-b"
		valid, err := ValidateOAuthJWT(token)
		if err == nil {
			t.Error("expected error for token signed with different secret")
		}
		if valid {
			t.Error("expected token to be invalid")
		}
	})
}

func TestGetEnvAsInt(t *testing.T) {
	const key = "TEST_INT_ENV_VAR"
	tests := []struct {
		name       string
		value      string
		set        bool
		defaultVal int
		want       int
	}{
		{"valid int", "42", true, 10, 42},
		{"invalid int falls back", "notanint", true, 10, 10},
		{"unset falls back", "", false, 7, 7},
		{"empty string falls back", "", true, 99, 99},
		{"negative int", "-3", true, 10, -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(key)
			if tt.set {
				os.Setenv(key, tt.value)
				defer os.Unsetenv(key)
			}
			if got := getEnvAsInt(key, tt.defaultVal); got != tt.want {
				t.Errorf("getEnvAsInt = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetEnvAsBool(t *testing.T) {
	const key = "TEST_BOOL_ENV_VAR"
	tests := []struct {
		name       string
		value      string
		set        bool
		defaultVal bool
		want       bool
	}{
		{"true", "true", true, false, true},
		{"false", "false", true, true, false},
		{"1 parses true", "1", true, false, true},
		{"0 parses false", "0", true, true, false},
		{"invalid falls back", "maybe", true, true, true},
		{"unset falls back", "", false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(key)
			if tt.set {
				os.Setenv(key, tt.value)
				defer os.Unsetenv(key)
			}
			if got := getEnvAsBool(key, tt.defaultVal); got != tt.want {
				t.Errorf("getEnvAsBool = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorMaps(t *testing.T) {
	// Every error with a status code should map to a sane HTTP code.
	for err, code := range ErrorStatusCodes {
		if err == nil {
			t.Error("nil error key in ErrorStatusCodes")
		}
		if code < 400 || code >= 600 {
			t.Errorf("error %v has non-error status code %d", err, code)
		}
	}
	if ErrorStatusCodes[ErrInvalidRequest] != 400 {
		t.Errorf("ErrInvalidRequest expected 400, got %d", ErrorStatusCodes[ErrInvalidRequest])
	}
	if ErrorStatusCodes[ErrServerError] != 500 {
		t.Errorf("ErrServerError expected 500, got %d", ErrorStatusCodes[ErrServerError])
	}
	if ErrInvalidCredentials.Error() != "invalid_credentials" {
		t.Errorf("unexpected ErrInvalidCredentials text: %q", ErrInvalidCredentials.Error())
	}
	if _, ok := ErrorDescriptions[ErrServerError]; !ok {
		t.Error("ErrServerError missing description")
	}
}
