package comindwork

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tusCapture holds the state captured from a test TUS server across the
// create (POST) and patch (PATCH) requests.
type tusCapture struct {
	createHeaders http.Header
	patchMethod   string
	patchPath     string
	patchHeaders  http.Header
	patchBody     []byte
}

// newTUSServer spins up a TUS test server that responds to the create step
// with the provided Location and to the patch step with 204 No Content.
// Pass locationHeader = "" to omit the Location header from the response.
func newTUSServer(t *testing.T, locationHeader string) (*httptest.Server, *tusCapture) {
	t.Helper()
	capture := &tusCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			capture.createHeaders = r.Header.Clone()
			if locationHeader != "" {
				w.Header().Set("Location", locationHeader)
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			capture.patchMethod = r.Method
			capture.patchPath = r.URL.Path
			capture.patchHeaders = r.Header.Clone()
			body, _ := io.ReadAll(r.Body)
			capture.patchBody = body
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, capture
}

func TestUploadFile_SendsTUSCreateHeaders(t *testing.T) {
	// Arrange
	srv, capture := newTUSServer(t, "/upload-tus/abc")
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, err := client.UploadFile(strings.NewReader("hello"), 5)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", capture.createHeaders.Get("Tus-Resumable"))
	assert.Equal(t, "5", capture.createHeaders.Get("Upload-Length"))
	assert.Equal(t, "application/offset+octet-stream", capture.createHeaders.Get("Content-Type"))
}

func TestUploadFile_ReturnsLocationHeaderVerbatim(t *testing.T) {
	// Arrange: a relative path, which is what the collection's example uses.
	srv, _ := newTUSServer(t, "/upload-tus/opaque-uid-xyz")
	client := NewClient(srv.URL, srv.Client())

	// Act
	uid, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/upload-tus/opaque-uid-xyz", uid)
}

func TestUploadFile_SendsPATCHWithBodyAndHeaders(t *testing.T) {
	// Arrange
	srv, capture := newTUSServer(t, "/upload-tus/abc")
	client := NewClient(srv.URL, srv.Client())
	body := "file-bytes-here"

	// Act
	_, err := client.UploadFile(strings.NewReader(body), int64(len(body)))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, capture.patchMethod)
	assert.Equal(t, "0", capture.patchHeaders.Get("Upload-Offset"))
	assert.Equal(t, "1.0.0", capture.patchHeaders.Get("Tus-Resumable"))
	assert.Equal(t, "application/offset+octet-stream", capture.patchHeaders.Get("Content-Type"))
	assert.Equal(t, body, string(capture.patchBody))
}

func TestUploadFile_ResolvesRelativeLocation(t *testing.T) {
	// Arrange: server returns a relative path-only Location.
	srv, capture := newTUSServer(t, "/upload-tus/rel-abc")
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/upload-tus/rel-abc", capture.patchPath)
}

func TestUploadFile_HandlesAbsoluteLocation(t *testing.T) {
	// Arrange: construct the server first so we can use its URL in Location.
	// We need an absolute Location, so build it dynamically inside the handler.
	capture := &tusCapture{}
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Location", srvURL+"/upload-tus/abs-xyz")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			capture.patchPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/upload-tus/abs-xyz", capture.patchPath)
}

func TestUploadFile_ReturnsErrorOnMissingLocation(t *testing.T) {
	// Arrange
	srv, _ := newTUSServer(t, "") // No Location header.
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	assert.Error(t, err)
}

func TestUploadFile_ReturnsErrorOnCreateFailure(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	assert.Error(t, err)
}

func TestUploadFile_ReturnsErrorOnPatchFailure(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Location", "/upload-tus/abc")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	assert.Error(t, err)
}

// --- DownloadFile ---

func TestDownloadFile_BuildsDownloadURLWithLMParam(t *testing.T) {
	// Arrange
	var gotMethod, gotPath, gotLM string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotLM = r.URL.Query().Get("lm")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, _, err := client.DownloadFile("f8f09e28-5b1a-4e8c-9d5e-2b3e6a9c0f1a")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/download/f8f09e28-5b1a-4e8c-9d5e-2b3e6a9c0f1a", gotPath)
	assert.Equal(t, "true", gotLM)
}

func TestDownloadFile_ReturnsBodyAndContentType(t *testing.T) {
	// Arrange
	wantBody := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(wantBody)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())

	// Act
	body, contentType, err := client.DownloadFile("some-id")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, wantBody, body)
	assert.Equal(t, "image/png", contentType)
}

func TestDownloadFile_AppliesAuthHeader(t *testing.T) {
	// Arrange
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("pdfbytes"))
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())
	client.SetAuthToken("dl-token")

	// Act
	_, _, err := client.DownloadFile("abc")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "CMW_AUTH_CODE dl-token", gotAuth)
}

func TestDownloadFile_ReturnsErrorOnEmptyID(t *testing.T) {
	// Arrange
	client := NewClient("http://example.com", nil)

	// Act
	_, _, err := client.DownloadFile("")

	// Assert
	assert.Error(t, err)
}

func TestDownloadFile_ReturnsErrorOnNon2xx(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())

	// Act
	_, _, err := client.DownloadFile("missing-id")

	// Assert
	assert.Error(t, err)
}

func TestUploadFile_AppliesAuthToBothRequests(t *testing.T) {
	// Arrange
	var createAuth, patchAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			createAuth = r.Header.Get("Authorization")
			w.Header().Set("Location", "/upload-tus/abc")
			w.WriteHeader(http.StatusCreated)
		case http.MethodPatch:
			patchAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, srv.Client())
	client.SetAuthToken("token-xyz")

	// Act
	_, err := client.UploadFile(strings.NewReader("x"), 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "CMW_AUTH_CODE token-xyz", createAuth)
	assert.Equal(t, "CMW_AUTH_CODE token-xyz", patchAuth)
}
