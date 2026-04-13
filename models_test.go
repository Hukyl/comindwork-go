package comindwork

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecord_GetString_ReturnsValue(t *testing.T) {
	// Arrange
	r := Record{"title": "hello"}

	// Act
	got := r.GetString("title")

	// Assert
	assert.Equal(t, "hello", got)
}

func TestRecord_GetString_ReturnsEmptyOnMissingKey(t *testing.T) {
	// Arrange
	r := Record{}

	// Act
	got := r.GetString("title")

	// Assert
	assert.Equal(t, "", got)
}

func TestRecord_GetString_ReturnsEmptyOnNonStringValue(t *testing.T) {
	// Arrange
	r := Record{"title": 42}

	// Act
	got := r.GetString("title")

	// Assert
	assert.Equal(t, "", got)
}

func TestRecord_GetFloat_ReturnsValue(t *testing.T) {
	// Arrange
	r := Record{"total": 3.5}

	// Act
	got := r.GetFloat("total")

	// Assert
	assert.Equal(t, 3.5, got)
}

func TestRecord_GetFloat_ReturnsZeroOnMissingKey(t *testing.T) {
	// Arrange
	r := Record{}

	// Act
	got := r.GetFloat("total")

	// Assert
	assert.Equal(t, 0.0, got)
}

func TestRecord_GetFloat_ReturnsZeroOnNonNumericValue(t *testing.T) {
	// Arrange
	r := Record{"total": "3.5"}

	// Act
	got := r.GetFloat("total")

	// Assert
	assert.Equal(t, 0.0, got)
}

func TestRecord_GetInt_TruncatesFloat(t *testing.T) {
	// Arrange: JSON numbers decode as float64.
	r := Record{"number": 42.9}

	// Act
	got := r.GetInt("number")

	// Assert
	assert.Equal(t, 42, got)
}

func TestRecord_GetInt_ReturnsZeroOnMissingKey(t *testing.T) {
	// Arrange
	r := Record{}

	// Act
	got := r.GetInt("number")

	// Assert
	assert.Equal(t, 0, got)
}
