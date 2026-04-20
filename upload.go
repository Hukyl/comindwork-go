package comindwork

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// DownloadFile fetches a file (attachment) body by its record UUID via
// GET /download/{id}?lm=true. Returns the full body and the server-reported
// Content-Type header. Symmetric to UploadFile.
//
// The lm=true query param mirrors the ComindWork UI and acts as a cache
// buster; it is set unconditionally and not exposed to callers.
func (c *APIClient) DownloadFile(id string) ([]byte, string, error) {
	if id == "" {
		return nil, "", fmt.Errorf("id is required")
	}
	resp, err := c.get(fmt.Sprintf("%s/download/%s?lm=true", c.baseURL, url.PathEscape(id)))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read download body: %w", err)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// TUS 1.0.0 protocol constants (https://tus.io).
const (
	tusVersion         = "1.0.0"
	tusContentType     = "application/offset+octet-stream"
	headerTusResumable = "Tus-Resumable"
	headerUploadLength = "Upload-Length"
	headerUploadOffset = "Upload-Offset"
	headerLocation     = "Location"
)

// UploadFile uploads a file to /upload-tus using the TUS 1.0.0 protocol in
// two requests: a POST that establishes the upload followed by a single PATCH
// that carries the full body.
//
// Returns the raw Location header value from the create step. Callers should
// use it as the file_uid (and id) field inside attachments when creating a
// record via Multi.
//
// Retries on transient failures are the caller's responsibility.
func (c *APIClient) UploadFile(r io.Reader, size int64) (string, error) {
	location, err := c.tusCreate(size)
	if err != nil {
		return "", fmt.Errorf("tus create: %w", err)
	}
	if err := c.tusPatch(location, r); err != nil {
		return "", fmt.Errorf("tus patch: %w", err)
	}
	return location, nil
}

// tusCreate performs POST /upload-tus and returns the Location header verbatim.
func (c *APIClient) tusCreate(size int64) (string, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/upload-tus", nil)
	if err != nil {
		return "", err
	}
	c.applyAuth(req)
	req.Header.Set(headerTusResumable, tusVersion)
	req.Header.Set(headerUploadLength, strconv.FormatInt(size, 10))
	req.Header.Set("Content-Type", tusContentType)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if isRespError(resp) {
		return "", fmt.Errorf("status %s", resp.Status)
	}

	location := resp.Header.Get(headerLocation)
	if location == "" {
		return "", fmt.Errorf("missing Location header in TUS create response")
	}
	return location, nil
}

// tusPatch uploads the body against the location returned by tusCreate. Per
// TUS 1.0.0 the Location may be absolute or relative; it is resolved against
// the client's base URL before the request is issued.
func (c *APIClient) tusPatch(location string, r io.Reader) error {
	patchURL, err := c.resolveLocation(location)
	if err != nil {
		return fmt.Errorf("resolve location: %w", err)
	}

	req, err := http.NewRequest("PATCH", patchURL, r)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	req.Header.Set(headerTusResumable, tusVersion)
	req.Header.Set(headerUploadOffset, "0")
	req.Header.Set("Content-Type", tusContentType)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if isRespError(resp) {
		return fmt.Errorf("status %s", resp.Status)
	}
	return nil
}

// resolveLocation resolves a TUS Location header value (absolute or relative)
// against the client's base URL per RFC 3986.
func (c *APIClient) resolveLocation(location string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}
