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
	"sync"
	"time"
)

// APIClient is the ComindWork (Extranet) API client
type APIClient struct {
	baseURL    string
	authToken  string
	client     *http.Client
	timeZone   string
	workspaces map[string]*Workspace
	mu         sync.RWMutex // Protects workspaces cache
}

// NewClient creates a new ComindWork API client
func NewClient(baseURL, timeZone string) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		client:     &http.Client{},
		timeZone:   timeZone,
		workspaces: make(map[string]*Workspace),
	}
}

// SetAuthToken sets the bearer token for API authentication
func (c *APIClient) SetAuthToken(token string) {
	c.authToken = token
}

// * HTTP methods utilities

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

// listURL builds a /tickets/list URL with query parameters from ListOptions.
func (c *APIClient) listURL(opts ListOptions) string {
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
	return fmt.Sprintf("%s/tickets/list?%s", c.baseURL, params.Encode())
}

// multiURL returns the URL for the /tickets/multi endpoint.
func (c *APIClient) multiURL() string {
	return fmt.Sprintf("%s/tickets/multi", c.baseURL)
}

// * Generic record operations

// ListRecords retrieves records using the global /tickets/list endpoint.
func (c *APIClient) ListRecords(opts ListOptions) ([]Record, error) {
	url := c.listURL(opts)
	resp, err := c.get(url)
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

// GetRecord retrieves a single record by its ID.
func (c *APIClient) GetRecord(id string, listOfFields string) (Record, error) {
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

// GetRecordByNumber retrieves a single record by its workspace-scoped number.
func (c *APIClient) GetRecordByNumber(workspaceAlias, appAlias string, number int, listOfFields string) (Record, error) {
	opts := ListOptions{
		ListOfFields: listOfFields,
		Filter:       fmt.Sprintf(`publishing_alias="%s" and project_alias="%s" and number=%d`, appAlias, workspaceAlias, number),
	}
	records, err := c.ListRecords(opts)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("record not found: %s/%s#%d", workspaceAlias, appAlias, number)
	}
	return records[0], nil
}

// CreateRecords creates one or more records via /tickets/multi.
// Each record should be a flat map with workspace_alias, app_alias, and field values.
func (c *APIClient) CreateRecords(records []map[string]any) ([]MultiResult, error) {
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

// UpdateRecords updates one or more records via /tickets/multi.
// Each record should have at least "id" and "transition" fields.
func (c *APIClient) UpdateRecords(records []map[string]any) ([]MultiResult, error) {
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

// * User operations

// ListUsers retrieves user records via the /aid/default-app-guser/tickets/list endpoint.
func (c *APIClient) ListUsers(opts ListOptions) ([]Record, error) {
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

	urlStr := fmt.Sprintf("%s/aid/%s/tickets/list?%s", c.baseURL, AppIDUser, params.Encode())

	resp, err := c.get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var records []Record
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode user records: %w", err)
	}
	return records, nil
}

// * Workspace operations

// RegisterWorkspace registers a workspace in the cache for lookup by alias
func (c *APIClient) RegisterWorkspace(workspace *Workspace) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspaces[workspace.Alias] = workspace
}

// GetWorkspaceByAlias returns a workspace by its alias from the cache
func (c *APIClient) GetWorkspaceByAlias(alias string) (*Workspace, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ws, ok := c.workspaces[alias]
	if !ok {
		return nil, fmt.Errorf("workspace with alias '%s' not found in cache", alias)
	}
	return ws, nil
}

// * Workday operations

// ListWorkdays retrieves workdays for a workspace within a date range
func (c *APIClient) ListWorkdays(workspaceAlias string, startDate, endDate time.Time) ([]Workday, error) {
	filter := fmt.Sprintf(
		`publishing_alias="%s" and project_alias="%s" and date>="%s" and date<="%s"`,
		AppAliasWorkday, workspaceAlias, FormatDate(startDate), FormatDate(endDate),
	)
	opts := ListOptions{
		ListOfFields: "ALL",
		Filter:       filter,
	}

	records, err := c.ListRecords(opts)
	if err != nil {
		return nil, err
	}

	workdays := make([]Workday, 0, len(records))
	for _, rec := range records {
		dateStr := rec.GetString("date")
		if dateStr == "" {
			slog.Warn("workday_missing_date", "record_id", rec.GetString("id"))
			continue
		}

		date, err := ParseDate(dateStr)
		if err != nil {
			slog.Warn("failed_to_parse_workday_date", "date", dateStr, "error", err)
			continue
		}

		workdays = append(workdays, Workday{
			ID:   rec.GetString("id"),
			Date: date,
		})
	}

	return workdays, nil
}

// GetOrCreateWorkday finds an existing workday for the given date or creates a new one
func (c *APIClient) GetOrCreateWorkday(workspaceAlias string, date time.Time) (*Workday, error) {
	workdays, err := c.ListWorkdays(workspaceAlias, date, date)
	if err != nil {
		return nil, err
	}

	dateStr := FormatDate(date)
	for _, wd := range workdays {
		if FormatDate(wd.Date) == dateStr {
			return &wd, nil
		}
	}

	return c.createWorkday(workspaceAlias, date)
}

// createWorkday creates a new workday for the given date
func (c *APIClient) createWorkday(workspaceAlias string, date time.Time) (*Workday, error) {
	record := map[string]any{
		"workspace_alias": workspaceAlias,
		"app_alias":       AppAliasWorkday,
		"date":            FormatDate(date),
	}

	results, err := c.CreateRecords([]map[string]any{record})
	if err != nil {
		return nil, fmt.Errorf("failed to create workday: %w", err)
	}

	if len(results) == 0 || !results[0].Successful {
		return nil, fmt.Errorf("failed to create workday: no successful result")
	}

	return &Workday{
		ID:   results[0].Data.GetString("id"),
		Date: date,
	}, nil
}

// * TimeLog operations

// CreateTimeLog creates a new timelog entry
func (c *APIClient) CreateTimeLog(workspaceAlias, workdayID string, startOn, finishOn time.Time, title string) (*TimeLog, error) {
	total := CalculateTotalReal(startOn, finishOn)

	record := map[string]any{
		"workspace_alias": workspaceAlias,
		"app_alias":       AppAliasTimelog,
		"title":           title,
		"start_on":        FormatISO8601(startOn),
		"finish_on":       FormatISO8601(finishOn),
		"total":           total,
	}

	results, err := c.CreateRecords([]map[string]any{record})
	if err != nil {
		return nil, fmt.Errorf("failed to create timelog: %w", err)
	}

	if len(results) == 0 || !results[0].Successful {
		return nil, fmt.Errorf("failed to create timelog: no successful result")
	}

	return &TimeLog{
		ID:        results[0].Data.GetString("id"),
		WorkdayID: workdayID,
		Title:     title,
		StartOn:   startOn,
		FinishOn:  finishOn,
		TotalReal: total,
	}, nil
}

// UpdateTimeLog updates an existing timelog entry
func (c *APIClient) UpdateTimeLog(id string, startOn, finishOn time.Time, title string) error {
	total := CalculateTotalReal(startOn, finishOn)

	record := map[string]any{
		"id":         id,
		"transition": TransitionEdit,
		"title":      title,
		"start_on":   FormatISO8601(startOn),
		"finish_on":  FormatISO8601(finishOn),
		"total":      total,
	}

	_, err := c.UpdateRecords([]map[string]any{record})
	return err
}

// DeleteTimeLog deletes a timelog entry
func (c *APIClient) DeleteTimeLog(id string) error {
	record := map[string]any{
		"id":         id,
		"transition": TransitionDelete,
	}

	_, err := c.UpdateRecords([]map[string]any{record})
	return err
}

// ListTimeLogs retrieves all timelogs for a workday
func (c *APIClient) ListTimeLogs(workspaceAlias, workdayID string) ([]TimeLog, error) {
	filter := fmt.Sprintf(`publishing_alias="%s" and project_alias="%s"`, AppAliasTimelog, workspaceAlias)
	opts := ListOptions{
		ListOfFields: "ALL",
		Filter:       filter,
	}

	records, err := c.ListRecords(opts)
	if err != nil {
		return nil, err
	}

	return c.parseTimeLogRecords(records, workdayID)
}

// ListTimeLogsByDateRange retrieves timelogs within a date range.
func (c *APIClient) ListTimeLogsByDateRange(startDate, endDate time.Time, opts ListOptions) ([]TimeLog, error) {
	dateFilter := fmt.Sprintf(
		`publishing_alias="%s" and start_on>"%s" and start_on<"%s"`,
		AppAliasTimelog,
		FormatISO8601(startDate),
		FormatISO8601(endDate),
	)

	if opts.Filter != "" {
		dateFilter = dateFilter + " and " + opts.Filter
	}

	listOpts := ListOptions{
		ListOfFields: opts.ListOfFields,
		LimitRecords: opts.LimitRecords,
		Filter:       dateFilter,
		SortBy:       opts.SortBy,
	}

	records, err := c.ListRecords(listOpts)
	if err != nil {
		return nil, err
	}

	return c.parseTimeLogRecords(records, "")
}

// parseTimeLogRecords converts raw API records into TimeLog structs.
func (c *APIClient) parseTimeLogRecords(records []Record, workdayID string) ([]TimeLog, error) {
	timeLogs := make([]TimeLog, 0, len(records))
	for _, rec := range records {
		startOnStr := rec.GetString("start_on")
		if startOnStr == "" {
			slog.Warn("timelog_missing_start_on", "record_id", rec.GetString("id"))
			continue
		}

		startOn, err := ParseISO8601(startOnStr)
		if err != nil {
			slog.Warn("failed_to_parse_timelog_start_on", "start_on", startOnStr, "error", err)
			continue
		}

		finishOnStr := rec.GetString("finish_on")
		if finishOnStr == "" {
			slog.Warn("timelog_missing_finish_on", "record_id", rec.GetString("id"))
			continue
		}

		finishOn, err := ParseISO8601(finishOnStr)
		if err != nil {
			slog.Warn("failed_to_parse_timelog_finish_on", "finish_on", finishOnStr, "error", err)
			continue
		}

		tl := TimeLog{
			ID:           rec.GetString("id"),
			WorkdayID:    workdayID,
			Title:        rec.GetString("title"),
			StartOn:      startOn,
			FinishOn:     finishOn,
			TotalReal:    rec.GetFloat("total"),
			ProjectAlias: rec.GetString("project_alias"),
			Number:       rec.GetInt("number"),
		}

		if updateDateStr := rec.GetString("real_update_date"); updateDateStr != "" {
			if ud, err := ParseISO8601(updateDateStr); err == nil {
				tl.UpdateDate = ud
			}
		} else if updateDateStr := rec.GetString("update_date"); updateDateStr != "" {
			if ud, err := ParseISO8601(updateDateStr); err == nil {
				tl.UpdateDate = ud
			}
		}

		timeLogs = append(timeLogs, tl)
	}

	return timeLogs, nil
}

// * Helper methods

// CreateTimeLogForDate is a convenience method that creates a timelog for a specific date,
// automatically finding or creating the workday first
func (c *APIClient) CreateTimeLogForDate(workspaceAlias string, date time.Time, startOn, finishOn time.Time, title string) (*TimeLog, error) {
	workday, err := c.GetOrCreateWorkday(workspaceAlias, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create workday: %w", err)
	}

	return c.CreateTimeLog(workspaceAlias, workday.ID, startOn, finishOn, title)
}

// ListTimeLogsForDate is a convenience method that lists all timelogs for a specific date
func (c *APIClient) ListTimeLogsForDate(workspaceAlias string, date time.Time) ([]TimeLog, error) {
	workdays, err := c.ListWorkdays(workspaceAlias, date, date)
	if err != nil {
		return nil, err
	}

	dateStr := FormatDate(date)
	for _, wd := range workdays {
		if FormatDate(wd.Date) == dateStr {
			return c.ListTimeLogs(workspaceAlias, wd.ID)
		}
	}

	// No workday found for this date, return empty list
	return []TimeLog{}, nil
}
