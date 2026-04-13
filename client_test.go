package comindwork

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient starts an httptest.Server with the given handler and returns
// an APIClient wired to it. The server is closed on test cleanup.
func newTestClient(t *testing.T, handler http.HandlerFunc) *APIClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, srv.Client())
}

func TestNewClient_NilHTTPClientUsesDefault(t *testing.T) {
	// Act
	client := NewClient("http://example.com", nil)

	// Assert
	require.NotNil(t, client.client)
}

func TestNewClient_UsesProvidedHTTPClient(t *testing.T) {
	// Arrange
	custom := &http.Client{}

	// Act
	client := NewClient("http://example.com", custom)

	// Assert
	assert.Same(t, custom, client.client)
}

func TestListRecords_BuildsGlobalListURL(t *testing.T) {
	// Arrange
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/tickets/list", gotPath)
}

func TestListRecords_EncodesAllListOptions(t *testing.T) {
	// Arrange
	var gotQuery = make(map[string]string)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.URL.Query() {
			gotQuery[k] = v[0]
		}
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{
		ListOfFields: "title,status",
		LimitRecords: 5,
		Filter:       `publishing_alias="TASK"`,
		SortBy:       "creation_date desc",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "title,status", gotQuery["listOfFields"])
	assert.Equal(t, "5", gotQuery["limitRecords"])
	assert.Equal(t, `publishing_alias="TASK"`, gotQuery["rlx"])
	assert.Equal(t, "creation_date desc", gotQuery["sortby"])
}

func TestListRecords_OmitsEmptyOptions(t *testing.T) {
	// Arrange
	var gotRawQuery string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, gotRawQuery)
}

func TestListRecords_OmitsLimitWhenZero(t *testing.T) {
	// Arrange
	var gotQuery = make(map[string][]string)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{ListOfFields: "title"})

	// Assert
	require.NoError(t, err)
	assert.NotContains(t, gotQuery, "limitRecords")
}

func TestListRecords_DecodesRecordsFromResponse(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"abc","title":"hi"},{"id":"def","title":"yo"}]`))
	})

	// Act
	records, err := client.ListRecords(ListOptions{})

	// Assert
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "abc", records[0].GetString("id"))
	assert.Equal(t, "yo", records[1].GetString("title"))
}

func TestListRecords_ReturnsErrorOnNon2xx(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Act
	_, err := client.ListRecords(ListOptions{})

	// Assert
	assert.Error(t, err)
}

func TestListRecords_ReturnsErrorOnMalformedJSON(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{})

	// Assert
	assert.Error(t, err)
}

func TestListRecordsInApp_BuildsScopedURL(t *testing.T) {
	// Arrange
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecordsInApp("E2E", "TASK", ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/w/E2E/a/TASK/tickets/list", gotPath)
}

func TestListRecordsByAppID_BuildsAppIDURL(t *testing.T) {
	// Arrange
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecordsByAppID("default-app-guser", ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/aid/default-app-guser/tickets/list", gotPath)
}

func TestGetRecord_FiltersByID(t *testing.T) {
	// Arrange
	var gotRLX string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRLX = r.URL.Query().Get("rlx")
		_, _ = w.Write([]byte(`[{"id":"abc","title":"x"}]`))
	})

	// Act
	rec, err := client.GetRecord("abc", "title")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, `id="abc"`, gotRLX)
	assert.Equal(t, "x", rec.GetString("title"))
}

func TestGetRecord_ReturnsErrorWhenNotFound(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.GetRecord("missing", "title")

	// Assert
	assert.Error(t, err)
}

func TestGetRecordByNumber_UsesScopedURLAndNumberFilter(t *testing.T) {
	// Arrange
	var gotPath, gotRLX string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRLX = r.URL.Query().Get("rlx")
		_, _ = w.Write([]byte(`[{"id":"abc","number":167}]`))
	})

	// Act
	rec, err := client.GetRecordByNumber("NICI", "TASK", 167, "ALL")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/w/NICI/a/TASK/tickets/list", gotPath)
	assert.Equal(t, "number=167", gotRLX)
	assert.Equal(t, 167, rec.GetInt("number"))
}

func TestGetRecordByNumber_ReturnsErrorWhenNotFound(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.GetRecordByNumber("NICI", "TASK", 999, "ALL")

	// Assert
	assert.Error(t, err)
}

func TestMulti_POSTsToMultiEndpoint(t *testing.T) {
	// Arrange
	var gotMethod, gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"successful":true}]`))
	})

	// Act
	_, err := client.Multi([]map[string]any{{"workspace_alias": "E2E", "app_alias": "TASK", "title": "x"}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/tickets/multi", gotPath)
}

func TestMulti_SendsRecordsAsJSONArray(t *testing.T) {
	// Arrange
	var gotBody []map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`[{"successful":true},{"successful":true}]`))
	})

	// Act
	_, err := client.Multi([]map[string]any{
		{"workspace_alias": "E2E", "app_alias": "TASK", "title": "first"},
		{"id": "abc", "transition": TransitionEdit, "title": "second"},
	})

	// Assert
	require.NoError(t, err)
	require.Len(t, gotBody, 2)
	assert.Equal(t, "first", gotBody[0]["title"])
	assert.Equal(t, "edit", gotBody[1]["transition"])
}

func TestMulti_SetsJSONContentType(t *testing.T) {
	// Arrange
	var gotContentType string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`[{"successful":true}]`))
	})

	// Act
	_, err := client.Multi([]map[string]any{{"title": "x"}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "application/json", gotContentType)
}

func TestMulti_DecodesMultiResults(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"successful":true,"created":true,"data":{"id":"abc"}},
			{"successful":false,"warnings":[{"message":"nope","severity":2}]}
		]`))
	})

	// Act
	results, err := client.Multi([]map[string]any{{"title": "a"}, {"title": "b"}})

	// Assert
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].Successful)
	assert.True(t, results[0].Created)
	assert.Equal(t, "abc", results[0].Data.GetString("id"))
	assert.False(t, results[1].Successful)
	require.Len(t, results[1].Warnings, 1)
	assert.Equal(t, "nope", results[1].Warnings[0].Message)
	assert.Equal(t, 2, results[1].Warnings[0].Severity)
}

func TestSetAuthToken_AddsAuthorizationHeader(t *testing.T) {
	// Arrange
	var gotAuth string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("[]"))
	})
	client.SetAuthToken("secret-token")

	// Act
	_, err := client.ListRecords(ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "CMW_AUTH_CODE secret-token", gotAuth)
}

func TestApplyAuth_OmitsAuthorizationHeaderWhenTokenEmpty(t *testing.T) {
	// Arrange
	var gotAuth string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("[]"))
	})

	// Act: no SetAuthToken call.
	_, err := client.ListRecords(ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, gotAuth)
}
