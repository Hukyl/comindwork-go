package comindwork

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

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
	var gotRLX, gotSkip string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRLX = r.URL.Query().Get("rlx")
		gotSkip = r.URL.Query().Get("skipAncestors")
		_, _ = w.Write([]byte(`[{"id":"abc","title":"x"}]`))
	})

	// Act
	rec, err := client.GetRecord("abc", "title")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, `id="abc"`, gotRLX)
	assert.Equal(t, "true", gotSkip)
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
	var gotPath, gotRLX, gotSkip string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRLX = r.URL.Query().Get("rlx")
		gotSkip = r.URL.Query().Get("skipAncestors")
		_, _ = w.Write([]byte(`[{"id":"abc","number":167}]`))
	})

	// Act
	rec, err := client.GetRecordByNumber("NICI", "TASK", 167, "ALL")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/w/NICI/a/TASK/tickets/list", gotPath)
	assert.Equal(t, `number="167"`, gotRLX)
	assert.Equal(t, "true", gotSkip)
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

// --- ListOptions extensions ---

func TestListRecords_EncodesSkipAncestorsWhenTrue(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{SkipAncestors: true})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "true", gotQuery.Get("skipAncestors"))
}

func TestListRecords_OmitsSkipAncestorsWhenFalse(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{SkipAncestors: false})

	// Assert
	require.NoError(t, err)
	assert.False(t, gotQuery.Has("skipAncestors"))
}

func TestListRecords_EncodesIncludeDeletedAfterAsISO8601UTC(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Act
	_, err := client.ListRecords(ListOptions{IncludeDeletedAfter: cutoff})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "2026-01-01T00:00:00.000Z", gotQuery.Get("includeDeletedAfter"))
}

func TestListRecords_OmitsIncludeDeletedAfterWhenZero(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.False(t, gotQuery.Has("includeDeletedAfter"))
}

func TestListRecords_MergesExtraParams(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{Extra: url.Values{"customFlag": []string{"yes"}}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "yes", gotQuery.Get("customFlag"))
}

func TestListRecords_ExtraOverridesNamedFieldOnConflict(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act: set skipAncestors via both channels; Extra must win.
	_, err := client.ListRecords(ListOptions{
		SkipAncestors: true,
		Extra:         url.Values{"skipAncestors": []string{"false"}},
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "false", gotQuery.Get("skipAncestors"))
}

// --- UsePOST form encoding ---

func TestListRecords_UsePOSTSendsFormEncodedBody(t *testing.T) {
	// Arrange
	var gotMethod, gotContentType, gotBody string
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecords(ListOptions{
		UsePOST:      true,
		ListOfFields: "title",
		LimitRecords: 3,
		Filter:       `publishing_alias="TASK"`,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	// Params must be in the body, not the URL.
	assert.Empty(t, gotQuery, "UsePOST should not leak params into the query string")
	// Body should decode as the same url.Values.
	parsedBody, err := url.ParseQuery(gotBody)
	require.NoError(t, err)
	assert.Equal(t, "title", parsedBody.Get("listOfFields"))
	assert.Equal(t, "3", parsedBody.Get("limitRecords"))
	assert.Equal(t, `publishing_alias="TASK"`, parsedBody.Get("rlx"))
}

func TestListRecordsInApp_UsePOSTHitsSameScopedPath(t *testing.T) {
	// Arrange
	var gotPath, gotMethod string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListRecordsInApp("E2E", "TASK", ListOptions{UsePOST: true})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/w/E2E/a/TASK/tickets/list", gotPath)
}

// --- GetCommon ---

func TestGetCommon_BuildsApialphaURL(t *testing.T) {
	// Arrange
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"user_name":"x"}`))
	})

	// Act
	_, err := client.GetCommon()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/apialpha.ashx/common", gotPath)
}

func TestGetCommon_DecodesRecord(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user_name":"Andrii","tz":100}`))
	})

	// Act
	rec, err := client.GetCommon()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Andrii", rec.GetString("user_name"))
	assert.Equal(t, 100, rec.GetInt("tz"))
}

func TestGetCommon_ReturnsErrorOnNon2xx(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Act
	_, err := client.GetCommon()

	// Assert
	assert.Error(t, err)
}

// --- ListHistoryInApp ---

func TestListHistoryInApp_BuildsScopedHistoryURL(t *testing.T) {
	// Arrange
	var gotMethod, gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListHistoryInApp("NICI", "TASK", ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/w/NICI/a/TASK/tickets/history", gotPath)
}

func TestListHistoryInApp_EncodesListOptionsQuery(t *testing.T) {
	// Arrange
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListHistoryInApp("NICI", "TASK", ListOptions{
		ListOfFields: "ALL",
		Filter:       `id="047695c5-8f48-4d6a-9b6c-9c6c39f4de4d"`,
		SortBy:       "version_timestamp desc",
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ALL", gotQuery.Get("listOfFields"))
	assert.Equal(t, `id="047695c5-8f48-4d6a-9b6c-9c6c39f4de4d"`, gotQuery.Get("rlx"))
	assert.Equal(t, "version_timestamp desc", gotQuery.Get("sortby"))
}

func TestListHistoryInApp_AppliesAuthHeader(t *testing.T) {
	// Arrange
	var gotAuth string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("[]"))
	})
	client.SetAuthToken("hist-token")

	// Act
	_, err := client.ListHistoryInApp("NICI", "TASK", ListOptions{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "CMW_AUTH_CODE hist-token", gotAuth)
}

func TestListHistoryInApp_DecodesHistoryEntries(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"version_id":"v1","transition":"comment","comment":"<p>hi</p>"},
			{"version_id":"v2","transition":"edit","minor_change":false}
		]`))
	})

	// Act
	records, err := client.ListHistoryInApp("NICI", "TASK", ListOptions{})

	// Assert
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "v1", records[0].GetString("version_id"))
	assert.Equal(t, "comment", records[0].GetString("transition"))
	assert.Equal(t, "edit", records[1].GetString("transition"))
}

func TestListHistoryInApp_UsePOSTHitsSameScopedPath(t *testing.T) {
	// Arrange
	var gotPath, gotMethod string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_, _ = w.Write([]byte("[]"))
	})

	// Act
	_, err := client.ListHistoryInApp("NICI", "TASK", ListOptions{UsePOST: true})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/w/NICI/a/TASK/tickets/history", gotPath)
}

func TestListHistoryInApp_ReturnsErrorOnNon2xx(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Act
	_, err := client.ListHistoryInApp("NICI", "TASK", ListOptions{})

	// Assert
	assert.Error(t, err)
}

// --- CountChanged ---

func TestCountChangedInApp_BuildsScopedURLAndSinceTime(t *testing.T) {
	// Arrange
	var gotPath, gotMethod, gotSince string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotSince = r.URL.Query().Get("sinceTime")
		_, _ = w.Write([]byte(`{"status":"ok","data":42}`))
	})
	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// Act
	n, err := client.CountChangedInApp("NICI", "TASK", since)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/apialpha.ashx/w/NICI/a/TASK/tickets/changed", gotPath)
	assert.Equal(t, "2026-04-01T00:00:00.000Z", gotSince)
	assert.Equal(t, 42, n)
}

func TestCountChangedByAppID_BuildsAppIDURL(t *testing.T) {
	// Arrange
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"status":"ok","data":7}`))
	})

	// Act
	n, err := client.CountChangedByAppID("v2-app-ni-task", time.Now())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/apialpha.ashx/aid/v2-app-ni-task/tickets/changed", gotPath)
	assert.Equal(t, 7, n)
}

func TestCountChangedInApp_EmitsOnlySinceTime(t *testing.T) {
	// Arrange: the API returns 500 if checksum/ts/layout_media are sent,
	// so the library must not include them.
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"status":"ok","data":0}`))
	})

	// Act
	_, err := client.CountChangedInApp("NICI", "TASK", time.Now())

	// Assert
	require.NoError(t, err)
	assert.Len(t, gotQuery, 1)
	assert.True(t, gotQuery.Has("sinceTime"))
}

func TestCountChangedInApp_ErrorsWhenStatusNotOK(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"error","data":0}`))
	})

	// Act
	_, err := client.CountChangedInApp("NICI", "TASK", time.Now())

	// Assert
	assert.Error(t, err)
}

func TestCountChangedInApp_ReturnsErrorOnNon2xx(t *testing.T) {
	// Arrange
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Act
	_, err := client.CountChangedInApp("NICI", "TASK", time.Now())

	// Assert
	assert.Error(t, err)
}
