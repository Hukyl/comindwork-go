package comindwork

import (
	"net/url"
	"time"
)

// AuthPrefix is the authorization scheme used by the ComindWork API.
const AuthPrefix = "CMW_AUTH_CODE"

// Transitions for /tickets/multi operations.
const (
	TransitionAdd    = "add"
	TransitionEdit   = "edit"
	TransitionDelete = "delete"
)

// Record is a generic API record returned as a flat JSON object with
// snake_case field names. Field layouts are organization- and app-specific;
// the ComindWork API does not expose a schema endpoint, so callers are
// responsible for knowing which fields a given app exposes.
type Record map[string]any

// GetString extracts a string field from the record.
func (r Record) GetString(key string) string {
	v, ok := r[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// GetFloat extracts a float64 field from the record.
func (r Record) GetFloat(key string) float64 {
	v, ok := r[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return f
}

// GetInt extracts an integer field from the record (JSON numbers are float64).
func (r Record) GetInt(key string) int {
	return int(r.GetFloat(key))
}

// ListOptions encapsulates query parameters for /tickets/list endpoints.
//
// The server accepts the query either as a URL query string (GET, default)
// or as an application/x-www-form-urlencoded POST body (set UsePOST to true
// to work around URL-length limits on long rlx expressions).
type ListOptions struct {
	ListOfFields        string     // Comma-separated field names, or "ALL"
	LimitRecords        int        // Max number of records to return (0 = server default)
	Filter              string     // rlx filter expression
	SortBy              string     // Sort expression, e.g. "creation_date desc"
	SkipAncestors       bool       // Emits skipAncestors=true when set.
	IncludeDeletedAfter time.Time  // Emits includeDeletedAfter=<ISO8601> when non-zero.
	Extra               url.Values // Arbitrary extra params; keys overwrite named fields on conflict.
	UsePOST             bool       // When true, encode params as a POST form body instead of a GET query string.
}

// MultiResult represents a single item in the response array from /tickets/multi.
type MultiResult struct {
	Created       bool           `json:"created,omitempty"`
	Updated       bool           `json:"updated,omitempty"`
	Successful    bool           `json:"successful"`
	Data          Record         `json:"data,omitempty"`
	Warnings      []MultiWarning `json:"warnings,omitempty"`
	TransactionID string         `json:"transactionId,omitempty"`
}

// MultiWarning represents a warning in a MultiResult.
type MultiWarning struct {
	Message  string `json:"message"`
	Severity int    `json:"severity"`
}
