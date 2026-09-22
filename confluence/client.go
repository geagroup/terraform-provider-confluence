package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client provides a connection to the Confluence API
type Client struct {
	client    *http.Client
	baseURL   *url.URL
	basePath  string
	publicURL *url.URL
	cloudID   string
	user      string
	token     string
}

// NewClientInput provides information to connect to the Confluence API
type NewClientInput struct {
	site                string
	siteScheme          string
	publicSite          string
	publicSiteScheme    string
	context             string
	cloudID             string
	user                string
	token               string
	allowPrivateSite    bool
	allowUnverifiedSite bool
}

// ErrorResponse describes why a request failed
type ErrorResponse struct {
	StatusCode int `json:"statusCode,omitempty"`
	Data       struct {
		Authorized bool     `json:"authorized,omitempty"`
		Valid      bool     `json:"valid,omitempty"`
		Errors     []string `json:"errors,omitempty"`
		Successful bool     `json:"successful,omitempty"`
	} `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// NewClient returns an authenticated client ready to use
func NewClient(input *NewClientInput) (*Client, error) {
	baseURL, err := validatedSiteURL(input.siteScheme, input.site)
	if err != nil {
		return nil, fmt.Errorf("invalid site: %w", err)
	}
	if input.cloudID != "" && !strings.EqualFold(baseURL.Hostname(), "api.atlassian.com") {
		return nil, fmt.Errorf("site must be api.atlassian.com when cloud_id is configured")
	}
	if isAtlassianHost(baseURL.Hostname()) {
		if baseURL.Scheme != "https" {
			return nil, fmt.Errorf("atlassian sites must use https")
		}
	} else if !input.allowUnverifiedSite {
		return nil, fmt.Errorf("site %q is not a verified Atlassian domain; set allow_unverified_site to use a self-hosted Confluence server", baseURL.Hostname())
	}
	if !input.allowPrivateSite {
		if ip := net.ParseIP(baseURL.Hostname()); ip != nil && isPrivateAddress(ip) {
			return nil, fmt.Errorf("site %q resolves to a private or non-routable address; set allow_private_site to permit it", baseURL.Hostname())
		}
	}

	publicSite := input.site
	if input.publicSite != "" {
		publicSite = input.publicSite
	}
	publicURL, err := validatedSiteURL(input.publicSiteScheme, publicSite)
	if err != nil {
		return nil, fmt.Errorf("invalid public_site: %w", err)
	}

	basePath, err := validatedContext(input.context)
	if err != nil {
		return nil, err
	}

	// Default to /wiki if using Confluence Cloud
	if strings.HasSuffix(strings.ToLower(baseURL.Hostname()), ".atlassian.net") {
		basePath = "/wiki"
	}
	if input.cloudID != "" {
		if strings.ContainsAny(input.cloudID, `/\?#`) {
			return nil, fmt.Errorf("cloud_id must be a single URL path segment")
		}
		basePath = fmt.Sprintf("/ex/confluence/%s/", input.cloudID)
	}

	return &Client{
		client:    newHTTPClient(baseURL, input.allowPrivateSite, input.user, input.token),
		baseURL:   baseURL,
		basePath:  basePath,
		publicURL: publicURL,
		cloudID:   input.cloudID,
		user:      input.user,
		token:     input.token,
	}, nil
}

func validatedSiteURL(scheme, site string) (*url.URL, error) {
	if scheme != "https" && scheme != "http" {
		return nil, fmt.Errorf("scheme must be https or http")
	}
	if site == "" || strings.ContainsAny(site, `/\?#`) {
		return nil, fmt.Errorf("hostname must not be empty or contain a path, query, fragment, or backslash")
	}
	u, err := url.Parse(scheme + "://" + site)
	if err != nil {
		return nil, err
	}
	if u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("hostname is malformed")
	}
	return u, nil
}

func validatedContext(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("context must be an absolute URL path")
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid context: %w", err)
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") ||
		u.IsAbs() || u.Host != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("context must be an absolute URL path without a host, query, or fragment")
	}
	decodedPath, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("invalid context: %w", err)
	}
	cleaned := path.Clean(decodedPath)
	if cleaned != strings.TrimSuffix(decodedPath, "/") {
		return "", fmt.Errorf("context must not contain traversal or duplicate path segments")
	}
	return cleaned, nil
}

func isAtlassianHost(host string) bool {
	host = strings.ToLower(host)
	return host == "api.atlassian.com" || strings.HasSuffix(host, ".atlassian.net")
}

func newHTTPClient(baseURL *url.URL, allowPrivate bool, user, token string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = restrictedDialContext(allowPrivate)

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !strings.EqualFold(req.URL.Scheme, baseURL.Scheme) ||
				!strings.EqualFold(req.URL.Host, baseURL.Host) {
				return fmt.Errorf("refusing redirect to a different origin: %s", req.URL.Redacted())
			}
			req.SetBasicAuth(user, token)
			return nil
		},
	}
}

func restrictedDialContext(allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid request address %q: %w", address, err)
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("site %q did not resolve to an IP address", host)
		}
		for _, address := range addresses {
			if !allowPrivate && isPrivateAddress(address.IP) {
				return nil, fmt.Errorf("site %q resolves to a private or non-routable address", host)
			}
		}
		resolvedHost := addresses[0].IP.String()
		if addresses[0].Zone != "" {
			resolvedHost += "%" + addresses[0].Zone
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(resolvedHost, port))
	}
}

func isPrivateAddress(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() ||
		!ip.IsGlobalUnicast() || isSharedAddress(ip)
}

func isSharedAddress(ip net.IP) bool {
	_, sharedRange, _ := net.ParseCIDR("100.64.0.0/10")
	return sharedRange.Contains(ip)
}

// GetString uses the client to send a GET request and returns a string
func (c *Client) GetString(path string) (string, error) {
	body := new(bytes.Buffer)
	responseBody, err := c.doRaw("GET", path, "", body)
	if err != nil {
		return "", err
	}
	result := responseBody.String()
	return result, nil
}

// Get uses the client to send a GET request
func (c *Client) Get(path string, result interface{}) error {
	body := new(bytes.Buffer)
	return c.do("GET", path, "", body, result)
}

// Delete uses the client to send a DELETE request
func (c *Client) Delete(path string) error {
	body := new(bytes.Buffer)
	return c.do("DELETE", path, "", body, nil)
}

// Post uses the client to send a POST request
func (c *Client) Post(path string, body interface{}, result interface{}) error {
	b, err := jsonBytesBuffer(body)
	if err != nil {
		return err
	}
	return c.do("POST", path, "application/json", b, result)
}

// Put uses the client to send a PUT request
func (c *Client) Put(path string, body interface{}, result interface{}) error {
	b, err := jsonBytesBuffer(body)
	if err != nil {
		return err
	}
	return c.do("PUT", path, "application/json", b, result)
}

// PostForm uses the client to send a multi-part form POST request
func (c *Client) PostForm(path, filename, body string, result interface{}) error {
	b, ct, err := formBytesBuffer(filename, body)
	if err != nil {
		return err
	}
	return c.do("POST", path, ct, b, result)
}

// PutForm uses the client to send a multi-part form PUT request
func (c *Client) PutForm(path, filename, body string, result interface{}) error {
	b, ct, err := formBytesBuffer(filename, body)
	if err != nil {
		return err
	}
	return c.do("PUT", path, ct, b, result)
}

func jsonBytesBuffer(body interface{}) (*bytes.Buffer, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(bodyBytes), nil
}

func bytesBufferJSON(bodyBytes *bytes.Buffer, result interface{}) error {
	if result == nil {
		return nil
	}
	reader := bytes.NewReader(bodyBytes.Bytes())
	return json.NewDecoder(reader).Decode(&result)
}

// formBytesBuffer returns the body as a multi-part form and the content type
func formBytesBuffer(filename, body string) (*bytes.Buffer, string, error) {
	bodyBytes := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBytes)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	_, err = io.WriteString(part, body)
	if err != nil {
		return nil, "", err
	}
	err = writer.Close()
	if err != nil {
		return nil, "", err
	}
	return bodyBytes, writer.FormDataContentType(), nil
}

func (c *Client) do(method, path, contentType string, body *bytes.Buffer, result interface{}) error {
	responseBody, err := c.doRaw(method, path, contentType, body)
	if err != nil {
		return err
	}
	return bytesBufferJSON(responseBody, result)
}

// do uses the client to send a specified request
func (c *Client) doRaw(method, path, contentType string, body *bytes.Buffer) (*bytes.Buffer, error) {
	u, fullPath, err := c.requestURL(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, "/", body)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = c.baseURL.Scheme
	req.URL.Host = c.baseURL.Host
	req.URL.Path = u.Path
	req.URL.RawPath = u.RawPath
	req.URL.RawQuery = u.RawQuery
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.SetBasicAuth(c.user, c.token)
	req.Header.Add("X-Atlassian-Token", "nocheck")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	expectedStatusCode := map[string]int{
		"POST":   200,
		"PUT":    200,
		"GET":    200,
		"DELETE": 204,
	}
	if resp.StatusCode != expectedStatusCode[method] {
		var responseBody string
		var errResponse ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errResponse)
		if err != nil {
			responseBody = "Could not decode error"
		} else {
			responseBody = errResponse.String()
		}
		s := body.String()
		return nil, fmt.Errorf("%s\n\n%s %s\n%s\n\n%s",
			resp.Status, method, fullPath, s, responseBody)
	}
	result := new(bytes.Buffer)
	_, err = result.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) requestURL(requestPath string) (*url.URL, string, error) {
	if strings.Contains(requestPath, `\`) {
		return nil, "", fmt.Errorf("request path must not contain a backslash")
	}
	ref, err := url.Parse(requestPath)
	if err != nil {
		return nil, "", err
	}
	if !strings.HasPrefix(requestPath, "/") || strings.HasPrefix(requestPath, "//") ||
		ref.IsAbs() || ref.Host != "" || ref.User != nil || ref.Fragment != "" {
		return nil, "", fmt.Errorf("request path must be an absolute path on the configured site")
	}
	fullPath := strings.TrimRight(c.basePath, "/") + "/" + strings.TrimLeft(ref.Path, "/")
	u := *c.baseURL
	u.Path = fullPath
	u.RawQuery = ref.RawQuery
	return &u, fullPath, nil
}

func (e *ErrorResponse) String() string {
	d := e.Data
	var errorsString string
	if len(d.Errors) > 0 {
		errorsString = fmt.Sprintf("\n  * %s", strings.Join(d.Errors, "\n  * "))
	}
	return fmt.Sprintf("%s\nAuthorized: %t\nValid: %t\nSuccessful: %t%s",
		e.Message, d.Authorized, d.Valid, d.Successful, errorsString)
}

// URL returns the public URL for a given path
func (c *Client) URL(path string) string {
	u, err := c.publicURL.Parse(path)
	if err != nil {
		return ""
	}
	return u.String()
}
