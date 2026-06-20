package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jondalar/mailscreener/internal/lists"
)

func setup(t *testing.T) (*httptest.Server, *lists.Store) {
	t.Helper()
	st, err := lists.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv := httptest.NewServer(New(st, "secret", "test", &Health{}).Handler())
	t.Cleanup(srv.Close)
	return srv, st
}

func do(t *testing.T, srv *httptest.Server, method, path, token, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body != "" {
		r, err = http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	} else {
		r, err = http.NewRequest(method, srv.URL+path, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAuthRequired(t *testing.T) {
	srv, _ := setup(t)
	if resp := do(t, srv, "GET", "/status", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", resp.StatusCode)
	}
	if resp := do(t, srv, "GET", "/status", "wrong", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token: got %d, want 401", resp.StatusCode)
	}
	if resp := do(t, srv, "GET", "/status", "secret", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("good token: got %d, want 200", resp.StatusCode)
	}
}

func TestUIServedUnauthenticated(t *testing.T) {
	srv, _ := setup(t)
	resp := do(t, srv, "GET", "/", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Mail Screener") {
		t.Fatal("UI body missing title")
	}
}

func TestListCRUDAndIdempotent(t *testing.T) {
	srv, _ := setup(t)
	if resp := do(t, srv, "POST", "/lists/whitelist", "secret", `{"value":"A@B.com"}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("post: %d", resp.StatusCode)
	}
	// Idempotent second post.
	if resp := do(t, srv, "POST", "/lists/whitelist", "secret", `{"value":"a@b.com"}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("post2: %d", resp.StatusCode)
	}
	resp := do(t, srv, "GET", "/lists/whitelist", "secret", "")
	var got struct {
		Entries []string `json:"entries"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	if len(got.Entries) != 1 || got.Entries[0] != "a@b.com" {
		t.Fatalf("entries = %v, want [a@b.com]", got.Entries)
	}
	if resp := do(t, srv, "DELETE", "/lists/whitelist/a@b.com", "secret", ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if resp := do(t, srv, "GET", "/lists/bogus", "secret", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("bogus kind: %d, want 404", resp.StatusCode)
	}
}

func TestClassifyEndpoint(t *testing.T) {
	srv, st := setup(t)
	st.Add(lists.Blocklist, "spam@x.com", "seed")
	resp := do(t, srv, "POST", "/classify", "secret", `{"sender":"spam@x.com"}`)
	var got struct {
		Verdict string `json:"verdict"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Verdict != "block" {
		t.Fatalf("verdict = %q, want block", got.Verdict)
	}
}
