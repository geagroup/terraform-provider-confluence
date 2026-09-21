package confluence

import (
	"fmt"
)

// Body is part of Content
type Body struct {
	Storage *Storage `json:"storage,omitempty"`
}

type contentBodyRequest struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

type updateContentRequest struct {
	Id      string             `json:"id"`
	Status  string             `json:"status"`
	Title   string             `json:"title"`
	Body    contentBodyRequest `json:"body"`
	Version *Version           `json:"version,omitempty"`
}

type createContentRequest struct {
	SpaceId  string             `json:"spaceId"`
	Status   string             `json:"status"`
	Title    string             `json:"title"`
	ParentId string             `json:"parentId,omitempty"`
	Body     contentBodyRequest `json:"body"`
	Subtype  string             `json:"subtype"`
}

// Content is a primary resource in Confluence
type Content struct {
	Id        string           `json:"id,omitempty"`
	Type      string           `json:"type,omitempty"`
	Title     string           `json:"title,omitempty"`
	SpaceId   string           `json:"spaceId,omitempty"`
	Version   *Version         `json:"version,omitempty"`
	Body      *Body            `json:"body,omitempty"`
	Links     *ContentLinks    `json:"_links,omitempty"`
	Ancestors []*Content       `json:"ancestors,omitempty"`
	Metadata  *ContentMetadata `json:"metadata,omitempty"`
}

// ContentLinks is part of Content
type ContentLinks struct {
	Context string `json:"context,omitempty"`
	WebUI   string `json:"webui,omitempty"`
}

// Storage is part of Body
type Storage struct {
	Value          string `json:"value,omitempty"`
	Representation string `json:"representation,omitempty"`
}

// Version is part of Content
type Version struct {
	Number int `json:"number,omitempty"`
}

// ContentMetadata is part of Content
type ContentMetadata struct {
	Labels []*Label `json:"labels,omitempty"`
}

// Label is part of Metadata
type Label struct {
	Prefix string `json:"prefix,omitempty"`
	Name   string `json:"name,omitempty"`
}

func (c *Client) CreateContent(content *Content) (*Content, error) {
	request := createContentRequestFromContent(content)
	// requestBody, err := json.MarshalIndent(request, "", "  ")
	// if err != nil {
	// 	return nil, fmt.Errorf("marshal CreateContent request: %w", err)
	// }

	// log.Printf("[DEBUG] CreateContent request:\n%s", requestBody)

	var response Content
	if err := c.Post("/wiki/api/v2/pages", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func createContentRequestFromContent(content *Content) *createContentRequest {
	request := &createContentRequest{
		SpaceId: content.SpaceId,
		Status:  "current",
		Title:   content.Title,
		Body: contentBodyRequest{
			Representation: "storage",
			Value:          content.Body.Storage.Value,
		},
		Subtype: "page",
	}
	if len(content.Ancestors) > 0 {
		request.ParentId = content.Ancestors[len(content.Ancestors)-1].Id
	}
	return request
}

func updateContentRequestFromContent(content *Content) *updateContentRequest {
	request := &updateContentRequest{
		Id:     content.Id,
		Status: "current",
		Title:  content.Title,
		Body: contentBodyRequest{
			Representation: "storage",
			Value:          content.Body.Storage.Value,
		},
		Version: &Version{Number: content.Version.Number},
	}

	return request
}

func (c *Client) GetContent(id string) (*Content, error) {
	var response Content
	path := fmt.Sprintf("/wiki/api/v2/pages/%s?body-format=storage", id)
	if err := c.Get(path, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) UpdateContent(content *Content) (*Content, error) {
	var response Content
	content.Version.Number++
	request := updateContentRequestFromContent(content)
	path := fmt.Sprintf("/wiki/api/v2/pages/%s", content.Id)
	if err := c.Put(path, request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteContent(id string) error {
	path := fmt.Sprintf("/wiki/api/v2/pages/%s", id)
	if err := c.Delete(path); err != nil {
		return err
	}
	return nil
}
