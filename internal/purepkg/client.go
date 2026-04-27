// Package purepkg implements a small client for the PureLang package
// registry (purepkg). The registry is a thin layer on top of GitHub:
// each package version resolves to a Git URL and a tag/commit. The MVP
// supports three HTTP endpoints:
//
//	GET  {base}/packages/{name}                  -> list versions
//	GET  {base}/packages/{name}/versions/{v}     -> resolve to git ref
//	GET  {base}/packages/search?q=<query>        -> list matching names
//	POST {base}/packages/{name}/versions         -> publish (token auth)
//
// All endpoints exchange JSON. The default base URL is
// https://purepkg.dev/api but can be overridden through the
// PUREPKG_URL environment variable.
//
// The registry is intentionally Git-backed so that anything resolvable
// here can also be expressed as a raw `git = "..."` dependency without
// any registry at all.
package purepkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the public registry endpoint. It can be overridden
// via the PUREPKG_URL environment variable or by passing a custom base
// URL to NewClient.
const DefaultBaseURL = "https://purepkg.dev/api"

// Client talks to a purepkg registry over HTTP.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Token   string // for publish operations
}

// NewClient returns a Client using the env-resolved base URL.
func NewClient() *Client {
	base := os.Getenv("PUREPKG_URL")
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{
		BaseURL: strings.TrimRight(base, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Token:   os.Getenv("PUREPKG_TOKEN"),
	}
}

// PackageVersion is a registry record for a single published version.
type PackageVersion struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	GitURL   string `json:"git"`
	Tag      string `json:"tag"`     // git tag, empty if commit pinning
	Commit   string `json:"commit"`  // optional commit hash
	Manifest string `json:"manifest"` // optional encoded pure.toml
}

// PackageSummary is returned by search.
type PackageSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Latest      string `json:"latest"`
}

// Versions returns all known versions for a package, newest first.
func (c *Client) Versions(name string) ([]PackageVersion, error) {
	u := fmt.Sprintf("%s/packages/%s", c.BaseURL, url.PathEscape(name))
	var resp []PackageVersion
	if err := c.getJSON(u, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Resolve returns the registry record for a specific name@version.
func (c *Client) Resolve(name, version string) (*PackageVersion, error) {
	u := fmt.Sprintf("%s/packages/%s/versions/%s", c.BaseURL, url.PathEscape(name), url.PathEscape(version))
	var resp PackageVersion
	if err := c.getJSON(u, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Search returns packages matching a free-text query.
func (c *Client) Search(query string) ([]PackageSummary, error) {
	u := fmt.Sprintf("%s/packages/search?q=%s", c.BaseURL, url.QueryEscape(query))
	var resp []PackageSummary
	if err := c.getJSON(u, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Publish uploads a new version of a package. Requires a token.
func (c *Client) Publish(p PackageVersion) error {
	if c.Token == "" {
		return errors.New("PUREPKG_TOKEN environment variable must be set to publish")
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/packages/%s/versions", c.BaseURL, url.PathEscape(p.Name))
	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("publish failed: %s\n%s", resp.Status, string(raw))
	}
	return nil
}

func (c *Client) getJSON(url string, dst interface{}) error {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: %s\n%s", url, resp.Status, string(raw))
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%s: invalid JSON response: %v", url, err)
	}
	return nil
}
