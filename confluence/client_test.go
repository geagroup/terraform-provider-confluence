package confluence

import (
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
	if !strings.Contains(err.Error(), "site must have suffix atlassian.com") {
		t.Fatalf("unexpected error: %s", err)
	}
}
