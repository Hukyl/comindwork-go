package comindwork

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

// APIClient is the ComindWork (Extranet) API client.
type APIClient struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// NewClient creates a new ComindWork API client. baseURL must include the
// API path prefix (e.g. "https://extranet.example.com/api").
func NewClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// SetAuthToken sets the token sent in the "Authorization: CMW_AUTH_CODE <token>" header.
func (c *APIClient) SetAuthToken(token string) {
	c.authToken = token
}

// * HTTP utilities

func isRespError(resp *http.Response) bool {
	ok := resp.StatusCode < 400
	if !ok {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("error_reading_response_body", "error", err)
		}
		slog.Error("request_failed", "method", resp.Request.Method, "status", resp.Status, "body", string(body))
	}
	return !ok
}

func (c *APIClient) applyAuth(req *http.Request) {
	if c.authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", AuthPrefix, c.authToken))
	}
}

func (c *APIClient) get(urlStr string) (*http.Response, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	c.applyAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if isRespError(resp) {
		return nil, fmt.Errorf("failed to %s: %s", req.Method, resp.Status)
	}

	return resp, nil
}

func (c *APIClient) post(urlStr string, data any) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if isRespError(resp) {
		return nil, fmt.Errorf("failed to %s: %s", req.Method, resp.Status)
	}

	return resp, nil
}

// * URL builders

// listQueryParams encodes ListOptions as a URL query string (without the leading '?').
func listQueryParams(opts ListOptions) string {
	params := url.Values{}
	if opts.ListOfFields != "" {
		params.Set("listOfFields", opts.ListOfFields)
	}
	if opts.LimitRecords > 0 {
		params.Set("limitRecords", strconv.Itoa(opts.LimitRecords))
	}
	if opts.Filter != "" {
		params.Set("rlx", opts.Filter)
	}
	if opts.SortBy != "" {
		params.Set("sortby", opts.SortBy)
	}
	return params.Encode()
}

// listURL builds the global /tickets/list URL.
func (c *APIClient) listURL(opts ListOptions) string {
	return fmt.Sprintf("%s/tickets/list?%s", c.baseURL, listQueryParams(opts))
}

// scopedListURL builds the workspace- and app-scoped /w/{wsAlias}/a/{appAlias}/tickets/list URL.
func (c *APIClient) scopedListURL(wsAlias, appAlias string, opts ListOptions) string {
	return fmt.Sprintf("%s/w/%s/a/%s/tickets/list?%s", c.baseURL, wsAlias, appAlias, listQueryParams(opts))
}

// appIDListURL builds the app-ID-scoped /aid/{appID}/tickets/list URL.
func (c *APIClient) appIDListURL(appID string, opts ListOptions) string {
	return fmt.Sprintf("%s/aid/%s/tickets/list?%s", c.baseURL, appID, listQueryParams(opts))
}

// multiURL returns the URL for the /tickets/multi endpoint.
func (c *APIClient) multiURL() string {
	return fmt.Sprintf("%s/tickets/multi", c.baseURL)
}

// * Record read operations

// ListRecords retrieves records via the global /tickets/list endpoint. The
// caller is expected to scope the query via opts.Filter (for example
// `publishing_alias="TASK" and project_alias="E2E"`).
func (c *APIClient) ListRecords(opts ListOptions) ([]Record, error) {
	return c.listRecords(c.listURL(opts))
}

// ListRecordsInApp retrieves records scoped to a workspace and app via the
// /w/{wsAlias}/a/{appAlias}/tickets/list endpoint.
func (c *APIClient) ListRecordsInApp(wsAlias, appAlias string, opts ListOptions) ([]Record, error) {
	return c.listRecords(c.scopedListURL(wsAlias, appAlias, opts))
}

// ListRecordsByAppID retrieves records scoped by app ID via the
// /aid/{appID}/tickets/list endpoint.
func (c *APIClient) ListRecordsByAppID(appID string, opts ListOptions) ([]Record, error) {
	return c.listRecords(c.appIDListURL(appID, opts))
}

func (c *APIClient) listRecords(urlStr string) ([]Record, error) {
	resp, err := c.get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var records []Record
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode records: %w", err)
	}
	return records, nil
}

// GetRecord retrieves a single record by its ID via the global /tickets/list endpoint.
func (c *APIClient) GetRecord(id, listOfFields string) (Record, error) {
	opts := ListOptions{
		ListOfFields: listOfFields,
		Filter:       fmt.Sprintf(`id="%s"`, id),
	}
	records, err := c.ListRecords(opts)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("record not found: %s", id)
	}
	return records[0], nil
}

// GetRecordByNumber retrieves a single record by its workspace-scoped number
// via the /w/{wsAlias}/a/{appAlias}/tickets/list endpoint.
func (c *APIClient) GetRecordByNumber(wsAlias, appAlias string, number int, listOfFields string) (Record, error) {
	opts := ListOptions{
		ListOfFields: listOfFields,
		Filter:       fmt.Sprintf(`number=%d`, number),
	}
	records, err := c.ListRecordsInApp(wsAlias, appAlias, opts)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("record not found: %s/%s#%d", wsAlias, appAlias, number)
	}
	return records[0], nil
}

// * Record write operations

// Multi creates, edits, or deletes records via POST /tickets/multi. Each
// record is a flat map — for creation, set workspace_alias, app_alias, and
// field values; for edits/deletes, set "id" and "transition" (TransitionEdit
// or TransitionDelete).
func (c *APIClient) Multi(records []map[string]any) ([]MultiResult, error) {
	resp, err := c.post(c.multiURL(), records)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []MultiResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode multi response: %w", err)
	}
	return results, nil
}
