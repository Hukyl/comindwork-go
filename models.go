package comindwork

import (
	"fmt"
	"time"
)

// Auth
const (
	AuthPrefix = "CMW_AUTH_CODE"
)

// App aliases used in scoped URLs (/w/{ws}/a/{app}/...)
const (
	AppAliasTask    = "TASK"
	AppAliasTimelog = "TIMELOG"
	AppAliasWorkday = "WORKDAY"
	AppAliasUser    = "USER"
)

// App IDs used in API responses and record identification
const (
	AppIDTask    = "v2-app-task"
	AppIDTimelog = "v2-hr-timelog"
	AppIDWorkday = "v2-hr-workday"
	AppIDUser    = "default-app-guser"
)

// Transitions for /tickets/multi operations
const (
	TransitionAdd    = "add"
	TransitionEdit   = "edit"
	TransitionDelete = "delete"
)

// Workspace represents a ComindWork workspace/project
type Workspace struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
	Name  string `json:"name,omitempty"`
}

func (w Workspace) String() string {
	if w.Name != "" {
		return fmt.Sprintf("Workspace <%s>: %s (%s)", w.ID, w.Name, w.Alias)
	}
	return fmt.Sprintf("Workspace <%s>: %s", w.ID, w.Alias)
}

// Workday represents a daily container for timelogs
type Workday struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	Date        time.Time  `json:"date"`
	TimeLogs    []*TimeLog `json:"timeLogs,omitempty"`
}

func (w Workday) String() string {
	return fmt.Sprintf("Workday <%s>: %s", w.ID, w.Date.Format("2006-01-02"))
}

// TimeLog represents a time entry in ComindWork
type TimeLog struct {
	ID           string    `json:"id"`
	WorkdayID    string    `json:"workdayId"`
	WorkspaceID  string    `json:"workspaceId"`
	Title        string    `json:"title"`
	StartOn      time.Time `json:"startOn"`
	FinishOn     time.Time `json:"finishOn"`
	TotalReal    float64   `json:"totalReal"` // Duration in hours
	Task         *Task     `json:"task,omitempty"`
	UpdateDate   time.Time `json:"updateDate,omitempty"`
	ProjectAlias string    `json:"projectAlias,omitempty"`
	Number       int       `json:"number,omitempty"`
}

func (t TimeLog) String() string {
	return fmt.Sprintf("TimeLog <%s>: %s (%s - %s)",
		t.ID, t.Title,
		t.StartOn.Format("15:04"),
		t.FinishOn.Format("15:04"))
}

// Task represents a task reference parsed from "WORKSPACE/TASKnnn" format
type Task struct {
	WorkspaceAlias string `json:"workspaceAlias"`
	TaskNumber     int    `json:"taskNumber"`
	TaskID         string `json:"taskId,omitempty"`
}

func (t Task) String() string {
	return fmt.Sprintf("%s/TASK%d", t.WorkspaceAlias, t.TaskNumber)
}

// User represents a ComindWork user/account
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Title     string `json:"title,omitempty"`
}

func (u User) String() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Email
}

// Record represents a generic API record returned as a flat JSON object.
// The official API returns records with snake_case field names.
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
type ListOptions struct {
	ListOfFields string // Comma-separated field names, or "ALL"
	LimitRecords int    // Max number of records to return (0 = server default)
	Filter       string // rlx filter expression
	SortBy       string // Sort expression, e.g. "creation_date desc"
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
