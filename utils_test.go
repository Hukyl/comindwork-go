package comindwork

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatISO8601_ConvertsToUTC(t *testing.T) {
	// Arrange
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	local := time.Date(2026, 4, 13, 8, 30, 0, 0, loc) // 12:30 UTC

	// Act
	got := FormatISO8601(local)

	// Assert
	assert.Equal(t, "2026-04-13T12:30:00.000Z", got)
}

func TestFormatISO8601_IncludesMillisecondsWhenZero(t *testing.T) {
	// Arrange
	ts := time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC)

	// Act
	got := FormatISO8601(ts)

	// Assert
	assert.Equal(t, "2026-04-13T10:00:00.000Z", got)
}

func TestParseISO8601_ReturnsUTCTime(t *testing.T) {
	// Act
	got, err := ParseISO8601("2026-04-13T10:00:00.000Z")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC), got)
}

func TestParseISO8601_ParsesTimestampWithoutMilliseconds(t *testing.T) {
	// Act
	got, err := ParseISO8601("2026-04-13T15:00:00Z")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC), got)
}

func TestParseISO8601_ReturnsErrorOnMalformedInput(t *testing.T) {
	// Act
	_, err := ParseISO8601("not-a-timestamp")

	// Assert
	assert.Error(t, err)
}

func TestFormatDate_ReturnsISODate(t *testing.T) {
	// Arrange
	ts := time.Date(2026, 4, 13, 15, 45, 0, 0, time.UTC)

	// Act
	got := FormatDate(ts)

	// Assert
	assert.Equal(t, "2026-04-13", got)
}

func TestParseDate_ReturnsMidnightUTC(t *testing.T) {
	// Act
	got, err := ParseDate("2026-04-13")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC), got)
}

func TestParseDate_ReturnsErrorOnMalformedInput(t *testing.T) {
	// Act
	_, err := ParseDate("2026/04/13")

	// Assert
	assert.Error(t, err)
}
