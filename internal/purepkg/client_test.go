package purepkg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionsResolveSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/packages/math":
			_ = json.NewEncoder(w).Encode([]PackageVersion{
				{Name: "math", Version: "0.2.0", GitURL: "https://example.com/math.git", Tag: "v0.2.0"},
				{Name: "math", Version: "0.1.0", GitURL: "https://example.com/math.git", Tag: "v0.1.0"},
			})
		case r.Method == "GET" && r.URL.Path == "/packages/math/versions/0.2.0":
			_ = json.NewEncoder(w).Encode(PackageVersion{
				Name: "math", Version: "0.2.0", GitURL: "https://example.com/math.git", Tag: "v0.2.0",
			})
		case r.Method == "GET" && r.URL.Path == "/packages/search":
			_ = json.NewEncoder(w).Encode([]PackageSummary{
				{Name: "math", Description: "Math utilities", Latest: "0.2.0"},
			})
		case r.Method == "POST" && r.URL.Path == "/packages/math/versions":
			if r.Header.Get("Authorization") != "Bearer SECRET" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	vs, err := c.Versions("math")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 || vs[0].Version != "0.2.0" {
		t.Errorf("got %+v", vs)
	}
	one, err := c.Resolve("math", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if one.GitURL != "https://example.com/math.git" {
		t.Errorf("got %+v", one)
	}
	hits, err := c.Search("math")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Description, "Math") {
		t.Errorf("got %+v", hits)
	}

	c2 := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Token: "SECRET"}
	if err := c2.Publish(PackageVersion{Name: "math", Version: "0.2.0", GitURL: "https://example.com/math.git"}); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRequiresToken(t *testing.T) {
	c := &Client{BaseURL: "http://nowhere", HTTP: http.DefaultClient}
	if err := c.Publish(PackageVersion{Name: "x", Version: "0.1"}); err == nil {
		t.Errorf("expected error for missing token")
	}
}
