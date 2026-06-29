package graphql

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarshalGQLData(t *testing.T) {
	gql := NewSubscriberGql()

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		// contains is a substring expected in the processed output
		contains string
	}{
		{
			name:     "simple map",
			input:    map[string]string{"key": "value"},
			wantErr:  false,
			contains: `key`,
		},
		{
			name:     "string with quotes gets escaped",
			input:    map[string]string{"k": `a"b`},
			wantErr:  false,
			contains: `\\\"`,
		},
		{
			name:    "unmarshalable value",
			input:   make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gql.MarshalGQLData(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (output=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Output must be wrapped in quotes (result of strconv.Quote).
			if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
				t.Errorf("expected quoted output, got %q", got)
			}
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("expected output %q to contain %q", got, tt.contains)
			}
		})
	}
}

func TestMarshalGQLData_EscapingRule(t *testing.T) {
	// Verify the exact transformation: json marshal -> strconv.Quote -> replace \" with \\\"
	gql := NewSubscriberGql()
	got, err := gql.MarshalGQLData(map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// json => {"a":"b"} ; quote => "{\"a\":\"b\"}" ; replace \" => \\\"
	want := `"{\\\"a\\\":\\\"b\\\"}"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSendRequest(t *testing.T) {
	tests := []struct {
		name         string
		serverBody   string
		serverStatus int
		payload      []byte
		wantBody     string
	}{
		{
			name:       "echoes server response body",
			serverBody: `{"ok":true}`,
			payload:    []byte(`{"query":"x"}`),
			wantBody:   `{"ok":true}`,
		},
		{
			name:         "returns body even on non-200",
			serverBody:   `{"errors":"boom"}`,
			serverStatus: http.StatusInternalServerError,
			payload:      []byte(`{}`),
			wantBody:     `{"errors":"boom"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPayload []byte
			var gotContentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotContentType = r.Header.Get("Content-Type")
				gotPayload, _ = io.ReadAll(r.Body)
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}
				_, _ = w.Write([]byte(tt.serverBody))
			}))
			defer srv.Close()

			gql := NewSubscriberGql()
			body, err := gql.SendRequest(srv.URL, tt.payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if string(gotPayload) != string(tt.payload) {
				t.Errorf("server received payload %q, want %q", gotPayload, tt.payload)
			}
		})
	}
}

func TestSendRequest_ServerReceivesPostMethod(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	gql := NewSubscriberGql()
	if _, err := gql.SendRequest(srv.URL, []byte(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("method = %q, want POST", method)
	}
}

func TestSendRequest_ConnectionError(t *testing.T) {
	gql := NewSubscriberGql()
	// Closed server -> dial error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	if _, err := gql.SendRequest(url, []byte(`{}`)); err == nil {
		t.Error("expected connection error, got nil")
	}
}

func TestSendRequest_BadURL(t *testing.T) {
	gql := NewSubscriberGql()
	// Control character in URL makes http.NewRequest fail.
	if _, err := gql.SendRequest("http://\x7f", []byte(`{}`)); err == nil {
		t.Error("expected request build error, got nil")
	}
}

// Sanity check that MarshalGQLData output, once unwrapped, decodes back to JSON.
func TestMarshalGQLData_RoundTrip(t *testing.T) {
	gql := NewSubscriberGql()
	in := map[string]int{"n": 42}
	got, err := gql.MarshalGQLData(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Reverse: undo \\\" -> \" then unquote.
	reversed := strings.ReplaceAll(got, `\\\"`, `\"`)
	var unquoted string
	if err := json.Unmarshal([]byte(reversed), &unquoted); err != nil {
		t.Fatalf("failed to unquote: %v", err)
	}
	var out map[string]int
	if err := json.Unmarshal([]byte(unquoted), &out); err != nil {
		t.Fatalf("failed to decode inner json: %v", err)
	}
	if out["n"] != 42 {
		t.Errorf("round trip mismatch: got %v", out)
	}
}
