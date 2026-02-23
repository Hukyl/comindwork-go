package comindwork

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

const (
	// ISO8601Layout is the time format used by ComindWork API
	ISO8601Layout = "2006-01-02T15:04:05.000Z"
	// DateLayout is the date format used for workday dates
	DateLayout = "2006-01-02"
)

// FormatISO8601 formats a time.Time to the ISO8601 format expected by ComindWork API
func FormatISO8601(t time.Time) string {
	return t.UTC().Format(ISO8601Layout)
}

// ParseISO8601 parses a timestamp string from the ComindWork API
func ParseISO8601(s string) (time.Time, error) {
	return time.Parse(ISO8601Layout, s)
}

// FormatDate formats a time.Time to the date format expected by ComindWork API
func FormatDate(t time.Time) string {
	return t.Format(DateLayout)
}

// ParseDate parses a date string from the ComindWork API
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// CalculateTotalReal calculates the duration in hours between start and end times
func CalculateTotalReal(start, end time.Time) float64 {
	duration := end.Sub(start)
	return duration.Hours()
}

// taskRefRegex matches patterns like "NICI/TASK167" or "TNAI/TASK170"
var taskRefRegex = regexp.MustCompile(`^([A-Z]+)/TASK(\d+)$`)

// ParseTaskReference parses a task reference string like "NICI/TASK167"
// Returns the workspace alias and task number, or an error if the format is invalid
func ParseTaskReference(ref string) (workspaceAlias string, taskNumber int, err error) {
	matches := taskRefRegex.FindStringSubmatch(ref)
	if matches == nil {
		return "", 0, fmt.Errorf("invalid task reference format: %s (expected WORKSPACE/TASKnnn)", ref)
	}

	workspaceAlias = matches[1]
	taskNumber, err = strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid task number in reference %s: %w", ref, err)
	}

	return workspaceAlias, taskNumber, nil
}

// FormatTaskReference formats a workspace alias and task number into a task reference string
func FormatTaskReference(workspaceAlias string, taskNumber int) string {
	return fmt.Sprintf("%s/TASK%d", workspaceAlias, taskNumber)
}
