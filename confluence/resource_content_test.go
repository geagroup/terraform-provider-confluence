package confluence

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestCreateContentRequestFromResourceData(t *testing.T) {
	data := schema.TestResourceDataRaw(t, resourceContent().Schema, map[string]interface{}{
		"space":  "123456",
		"body":   "Content body",
		"title":  "Content title",
		"parent": "654321",
		"subtype": "page",
	})

	body, err := json.Marshal(createContentRequestFromContent(contentFromResourceData(data)))
	if err != nil {
		t.Fatalf("failed to marshal content: %s", err)
	}

	var request struct {
		SpaceId  string `json:"spaceId"`
		Status   string `json:"status"`
		Title    string `json:"title"`
		ParentId string `json:"parentId"`
		Body     struct {
			Representation string `json:"representation"`
			Value          string `json:"value"`
		} `json:"body"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("failed to unmarshal request body: %s", err)
	}
	if request.SpaceId != "123456" {
		t.Fatalf("spaceId = %q, want %q", request.SpaceId, "123456")
	}
	if request.Status != "current" {
		t.Fatalf("status = %q, want %q", request.Status, "current")
	}
	if request.Title != "Content title" {
		t.Fatalf("title = %q, want %q", request.Title, "Content title")
	}
	if request.ParentId != "654321" {
		t.Fatalf("parentId = %q, want %q", request.ParentId, "654321")
	}
	if request.Body.Representation != "storage" {
		t.Fatalf("body representation = %q, want %q", request.Body.Representation, "storage")
	}
	if request.Body.Value != "Content body" {
		t.Fatalf("body value = %q, want %q", request.Body.Value, "Content body")
	}
	if request.Subtype != "page" {
		t.Fatalf("subtype = %q, want %q", request.Subtype, "page")
	}
}

func TestAccConfluenceContent_Updated(t *testing.T) {
	rName := acctest.RandomWithPrefix("resource-content-test")
	resourceName := "confluence_content.default"
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckConfluenceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckConfluenceContentConfigRequired(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckConfluenceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "title", rName),
					resource.TestCheckResourceAttr(resourceName, "body", "Original value"),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
				),
			},
			{
				Config: testAccCheckConfluenceContentConfigUpdated(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckConfluenceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "title", rName),
					resource.TestCheckResourceAttr(resourceName, "body", "Updated value"),
					resource.TestCheckResourceAttr(resourceName, "version", "2"),
				),
			},
		},
	})
}

func TestAccConfluenceContent_Parent(t *testing.T) {
	rName := acctest.RandomWithPrefix("resource-content-test")
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckConfluenceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckConfluenceContentConfigParent(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckConfluenceExists("confluence_content.parent"),
					testAccCheckConfluenceExists("confluence_content.child"),
					resource.TestCheckResourceAttrPair("confluence_content.child", "parent",
						"confluence_content.parent", "id"),
				),
			},
		},
	})
}

func testAccCheckConfluenceContentConfigRequired(rName string) string {
	return fmt.Sprintf(`
resource confluence_content "default" {
  title = "%s"
  body  = "Original value"
}
`, rName)
}

func testAccCheckConfluenceContentConfigUpdated(rName string) string {
	return fmt.Sprintf(`
	resource confluence_content "default" {
		title = "%s"
		body  = "Updated value"
	}
	`, rName)
}

func testAccCheckConfluenceContentConfigParent(rName string) string {
	return fmt.Sprintf(`
	resource confluence_content "parent" {
		title = "%s-parent"
		body  = "parent"
	}
	resource confluence_content "child" {
		title  = "%s-child"
		body   = "child"
		parent = confluence_content.parent.id
	}
	`, rName, rName)
}
