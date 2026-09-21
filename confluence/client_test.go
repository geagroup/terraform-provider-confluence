package confluence

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewClientCloudID(t *testing.T) {
	client, err := NewClient(&NewClientInput{
		site:             "api.atlassian.com",
		siteScheme:       "https",
		publicSiteScheme: "https",
		cloudID:          "cloud-id",
	})
	if err != nil {
		t.Fatalf("NewClient returned an error: %s", err)
	}

	if got, want := client.basePath, "/ex/confluence/cloud-id/"; got != want {
		t.Fatalf("base path = %q, want %q", got, want)
	}
}

func TestNewClientCloudIDRejectsInvalidSite(t *testing.T) {
	_, err := NewClient(&NewClientInput{
		site:             "example.atlassian.net",
		siteScheme:       "https",
		publicSiteScheme: "https",
		cloudID:          "cloud-id",
	})
	if err == nil {
		t.Fatal("NewClient must reject a non-atlassian.com site when cloud_id is configured")
	}
	if !strings.Contains(err.Error(), "site must be api.atlassian.com") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestNewClientRejectsUnverifiedSite(t *testing.T) {
	_, err := NewClient(&NewClientInput{
		site:             "confluence.example.com",
		siteScheme:       "https",
		publicSiteScheme: "https",
	})
	if err == nil {
		t.Fatal("NewClient must reject an unverified site by default")
	}
	if !strings.Contains(err.Error(), "allow_unverified_site") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestNewClientRejectsPrivateSite(t *testing.T) {
	_, err := NewClient(&NewClientInput{
		site:                "127.0.0.1",
		siteScheme:          "http",
		publicSiteScheme:    "http",
		allowUnverifiedSite: true,
	})
	if err == nil {
		t.Fatal("NewClient must reject a private site by default")
	}
	if !strings.Contains(err.Error(), "allow_private_site") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestNewClientRejectsUnsafeContext(t *testing.T) {
	contexts := []string{
		"http://169.254.169.254/latest/meta-data/",
		"//169.254.169.254/latest/meta-data/",
		"/wiki?redirect=http://example.com",
		"/wiki#fragment",
		"/wiki/../admin",
		`/wiki\admin`,
	}
	for _, context := range contexts {
		t.Run(context, func(t *testing.T) {
			_, err := NewClient(&NewClientInput{
				site:             "example.atlassian.net",
				siteScheme:       "https",
				publicSiteScheme: "https",
				context:          context,
			})
			if err == nil {
				t.Fatalf("NewClient must reject unsafe context %q", context)
			}
		})
	}
}

func TestClientRejectsAbsoluteRequestPath(t *testing.T) {
	client, err := NewClient(&NewClientInput{
		site:             "example.atlassian.net",
		siteScheme:       "https",
		publicSiteScheme: "https",
	})
	if err != nil {
		t.Fatalf("NewClient returned an error: %s", err)
	}

	err = client.Get("http://169.254.169.254/latest/meta-data/", nil)
	if err == nil {
		t.Fatal("Get must reject an absolute request URL")
	}
	if !strings.Contains(err.Error(), "configured site") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestClientBlocksCrossOriginRedirect(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	client := newTestClient(t, source.URL)
	err := client.Get("/redirect", nil)
	if err == nil {
		t.Fatal("Get must reject a cross-origin redirect")
	}
	if !strings.Contains(err.Error(), "refusing redirect to a different origin") {
		t.Fatalf("unexpected error: %s", err)
	}
	if targetCalled {
		t.Fatal("redirect target must not receive a request")
	}
}

func TestClientSendsCredentialsToConfiguredOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, token, ok := r.BasicAuth()
		if !ok || user != "user" || token != "token" {
			http.Error(w, "missing credentials", http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	var result map[string]interface{}
	if err := client.Get("/test", &result); err != nil {
		t.Fatalf("Get returned an error: %s", err)
	}
}

func newTestClient(t *testing.T, rawURL string) *Client {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test server URL: %s", err)
	}
	client, err := NewClient(&NewClientInput{
		site:                u.Host,
		siteScheme:          u.Scheme,
		publicSiteScheme:    u.Scheme,
		user:                "user",
		token:               "token",
		allowPrivateSite:    true,
		allowUnverifiedSite: true,
	})
	if err != nil {
		t.Fatalf("NewClient returned an error: %s", err)
	}
	return client
}
