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
	"strings"
	"time"
)

// APIClient is the ComindWork (Extranet) API client.
type APIClient struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// NewClient creates a new ComindWork API client. baseURL must include the
// API path prefix (e.g. "https://extranet.example.com/api"). Pass nil for
// httpClient to use a default &http.Client{}; tests and callers that need
// custom transports, timeouts, or cookies should pass a configured client.
func NewClient(baseURL string, httpClient *http.Client) *APIClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &APIClient{
		baseURL: baseURL,
		client:  httpClient,
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

// postForm issues a POST with the given values encoded as
// application/x-www-form-urlencoded in the request body.
func (c *APIClient) postForm(urlStr string, values url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}

	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

// listQueryParams renders ListOptions as url.Values. Named fields are
// written first; Extra is merged in afterwards and overwrites on conflict.
func listQueryParams(opts ListOptions) url.Values {
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
	if opts.SkipAncestors {
		params.Set("skipAncestors", "true")
	}
	if !opts.IncludeDeletedAfter.IsZero() {
		params.Set("includeDeletedAfter", FormatISO8601(opts.IncludeDeletedAfter))
	}
	for key, vals := range opts.Extra {
		params.Del(key)
		for _, v := range vals {
			params.Add(key, v)
		}
	}
	return params
}

// listBaseURL is the global /tickets/list endpoint without query string.
func (c *APIClient) listBaseURL() string {
	return fmt.Sprintf("%s/tickets/list", c.baseURL)
}

// scopedListBaseURL is the workspace- and app-scoped /w/{wsAlias}/a/{appAlias}/tickets/list endpoint.
func (c *APIClient) scopedListBaseURL(wsAlias, appAlias string) string {
	return fmt.Sprintf("%s/w/%s/a/%s/tickets/list", c.baseURL, wsAlias, appAlias)
}

// scopedHistoryBaseURL is the workspace- and app-scoped /w/{wsAlias}/a/{appAlias}/tickets/history endpoint.
func (c *APIClient) scopedHistoryBaseURL(wsAlias, appAlias string) string {
	return fmt.Sprintf("%s/w/%s/a/%s/tickets/history", c.baseURL, wsAlias, appAlias)
}

// appIDListBaseURL is the /aid/{appID}/tickets/list endpoint.
func (c *APIClient) appIDListBaseURL(appID string) string {
	return fmt.Sprintf("%s/aid/%s/tickets/list", c.baseURL, appID)
}

// multiURL returns the URL for the /tickets/multi endpoint.
func (c *APIClient) multiURL() string {
	return fmt.Sprintf("%s/tickets/multi", c.baseURL)
}

// commonURL returns the /apialpha.ashx/common session endpoint.
func (c *APIClient) commonURL() string {
	return fmt.Sprintf("%s/apialpha.ashx/common", c.baseURL)
}

// scopedChangedURL is the workspace-and-app-scoped /apialpha.ashx/w/{ws}/a/{app}/tickets/changed endpoint.
func (c *APIClient) scopedChangedURL(wsAlias, appAlias string) string {
	return fmt.Sprintf("%s/apialpha.ashx/w/%s/a/%s/tickets/changed", c.baseURL, wsAlias, appAlias)
}

// appIDChangedURL is the /apialpha.ashx/aid/{appID}/tickets/changed endpoint.
func (c *APIClient) appIDChangedURL(appID string) string {
	return fmt.Sprintf("%s/apialpha.ashx/aid/%s/tickets/changed", c.baseURL, appID)
}

// * Record read operations

// ListRecords retrieves records via the global /tickets/list endpoint. The
// caller is expected to scope the query via opts.Filter (for example
// `publishing_alias="TASK" and project_alias="E2E"`).
func (c *APIClient) ListRecords(opts ListOptions) ([]Record, error) {
	return c.listRecords(c.listBaseURL(), opts)
}

// ListRecordsInApp retrieves records scoped to a workspace and app via the
// /w/{wsAlias}/a/{appAlias}/tickets/list endpoint.
func (c *APIClient) ListRecordsInApp(wsAlias, appAlias string, opts ListOptions) ([]Record, error) {
	return c.listRecords(c.scopedListBaseURL(wsAlias, appAlias), opts)
}

// ListRecordsByAppID retrieves records scoped by app ID via the
// /aid/{appID}/tickets/list endpoint.
func (c *APIClient) ListRecordsByAppID(appID string, opts ListOptions) ([]Record, error) {
	return c.listRecords(c.appIDListBaseURL(appID), opts)
}

// ListHistoryInApp retrieves version entries (edits, state changes, comments)
// for records scoped to a workspace and app via the
// /w/{wsAlias}/a/{appAlias}/tickets/history endpoint. The server accepts the
// same query shape as /tickets/list, so ListOptions (rlx, listOfFields,
// sortby, limitRecords, skipAncestors, UsePOST) is reused verbatim.
//
// Each returned Record is a versioned snapshot; interpretation of fields
// (version_id, version_timestamp, transition, comment, attachments__list,
// etc.) is the caller's responsibility.
func (c *APIClient) ListHistoryInApp(wsAlias, appAlias string, opts ListOptions) ([]Record, error) {
	return c.listRecords(c.scopedHistoryBaseURL(wsAlias, appAlias), opts)
}

func (c *APIClient) listRecords(baseURL string, opts ListOptions) ([]Record, error) {
	params := listQueryParams(opts)

	var resp *http.Response
	var err error
	if opts.UsePOST {
		resp, err = c.postForm(baseURL, params)
	} else {
		resp, err = c.get(baseURL + "?" + params.Encode())
	}
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
		ListOfFields:  listOfFields,
		Filter:        fmt.Sprintf(`id="%s"`, id),
		SkipAncestors: true,
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
// via the /w/{wsAlias}/a/{appAlias}/tickets/list endpoint. The number filter
// is emitted as number="<n>" to match the convention seen in production
// browser traffic; the server accepts unquoted numbers as well.
func (c *APIClient) GetRecordByNumber(wsAlias, appAlias string, number int, listOfFields string) (Record, error) {
	opts := ListOptions{
		ListOfFields:  listOfFields,
		Filter:        fmt.Sprintf(`number="%d"`, number),
		SkipAncestors: true,
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

// * Session / incremental sync

// GetCommon fetches session and workspace bootstrap info from
// GET /apialpha.ashx/common. The response contains identity (user_name,
// user id, email), locale and timezone settings, system configs, and
// translation strings — all organization-specific, returned as a raw Record.
func (c *APIClient) GetCommon() (Record, error) {
	resp, err := c.get(c.commonURL())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var record Record
	if err := json.NewDecoder(resp.Body).Decode(&record); err != nil {
		return nil, fmt.Errorf("failed to decode common response: %w", err)
	}
	return record, nil
}

// CountChangedInApp returns the number of records in the given workspace and
// app that have changed since sinceTime. Uses GET against
// /apialpha.ashx/w/{wsAlias}/a/{appAlias}/tickets/changed.
func (c *APIClient) CountChangedInApp(wsAlias, appAlias string, sinceTime time.Time) (int, error) {
	return c.countChanged(c.scopedChangedURL(wsAlias, appAlias), sinceTime)
}

// CountChangedByAppID returns the number of records in the given app (by ID)
// that have changed since sinceTime. Uses GET against
// /apialpha.ashx/aid/{appID}/tickets/changed.
func (c *APIClient) CountChangedByAppID(appID string, sinceTime time.Time) (int, error) {
	return c.countChanged(c.appIDChangedURL(appID), sinceTime)
}

func (c *APIClient) countChanged(baseURL string, sinceTime time.Time) (int, error) {
	params := url.Values{}
	params.Set("sinceTime", FormatISO8601(sinceTime))

	resp, err := c.get(baseURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var body struct {
		Status string `json:"status"`
		Data   int    `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("failed to decode changed response: %w", err)
	}
	if body.Status != "ok" {
		return 0, fmt.Errorf("unexpected changed response status: %q", body.Status)
	}
	return body.Data, nil
}
