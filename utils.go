package comindwork

import "time"

const (
	// ISO8601Layout is the timestamp format used by the ComindWork API.
	ISO8601Layout = "2006-01-02T15:04:05.000Z"
	// DateLayout is the date-only format used for date fields in the ComindWork API.
	DateLayout = "2006-01-02"
)

// FormatISO8601 formats a time.Time to the ISO8601 format expected by the ComindWork API.
func FormatISO8601(t time.Time) string {
	return t.UTC().Format(ISO8601Layout)
}

// ParseISO8601 parses a timestamp string from the ComindWork API.
// It accepts both millisecond-precision ("...T15:04:05.000Z") and
// second-precision ("...T15:04:05Z") timestamps.
func ParseISO8601(s string) (time.Time, error) {
	t, err := time.Parse(ISO8601Layout, s)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// FormatDate formats a time.Time to the date format expected by the ComindWork API.
func FormatDate(t time.Time) string {
	return t.Format(DateLayout)
}

// ParseDate parses a date string from the ComindWork API.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}
