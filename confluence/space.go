package confluence

import (
	"fmt"
	"strings"
)

// Content is a primary resource in Confluence
type Space struct {
	Id    int         `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Key   string      `json:"key,omitempty"`
	Links *SpaceLinks `json:"_links,omitempty"`
}

// ContentLinks is part of Content
type SpaceLinks struct {
	Base  string `json:"base,omitempty"`
	WebUI string `json:"webui,omitempty"`
}

func (c *Client) CreateSpace(space *Space) (*Space, error) {
	var response Space
	if err := c.Post("/wiki/api/space", space, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetSpace(id string) (*Space, error) {
	var response Space
	path := fmt.Sprintf("/wiki/api/space/%s", id)
	if err := c.Get(path, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) UpdateSpace(space *Space) (*Space, error) {
	var response Space

	path := fmt.Sprintf("/wiki/api/space/%s", space.Key)
	if err := c.Put(path, space, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteSpace(id string) error {
	path := fmt.Sprintf("/wiki/api/space/%s", id)
	if err := c.Delete(path); err != nil {
		if strings.HasPrefix(err.Error(), "202 ") {
			//202 is the delete API success response
			//Other APIs return 204. Because, reasons.
			return nil
		}
		return err
	}
	return nil
}
