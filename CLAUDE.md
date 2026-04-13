# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go client library for the [ComindWork](https://comindwork.com) (Extranet) REST API. Zero runtime dependencies (stdlib only); `github.com/stretchr/testify` is a test-only dep. Module: `github.com/Hukyl/comindwork-go`, requires Go 1.22+.

The library is a thin transport for the ComindWork metaframework: it exposes generic record CRUD plus TUS file upload. Apps (TASK, TIMELOG, WORKDAY, etc.) and their field layouts are organization-specific and are the **caller's** responsibility — the API has no schema introspection endpoint, so this library deliberately ships no typed app models.

## Commands

```bash
go build ./...                # Build
go test ./...                 # Run all tests
go test -run TestFoo ./...    # Run a single test
go test -cover ./...          # Coverage summary
go vet ./...                  # Static analysis
```

No Makefile, no linter config, no CI pipeline exists yet.

## Architecture

Single-package library (`package comindwork`) with four files:

- **`client.go`** — `APIClient` struct and record operations. Layers: unexported HTTP helpers (`get`, `post`, `postForm`, `applyAuth`, `isRespError`) → unexported URL builders (`listBaseURL`, `scopedListBaseURL`, `appIDListBaseURL`, `multiURL`, `commonURL`, `scopedChangedURL`, `appIDChangedURL`, `listQueryParams`) → public operations: record reads (`ListRecords`, `ListRecordsInApp`, `ListRecordsByAppID`, `GetRecord`, `GetRecordByNumber`), record writes (`Multi`), session / incremental sync (`GetCommon`, `CountChangedInApp`, `CountChangedByAppID`).
- **`upload.go`** — TUS 1.0.0 file upload (`UploadFile`) and internal `tusCreate`/`tusPatch`/`resolveLocation` helpers.
- **`models.go`** — Generic types (`Record`, `ListOptions`, `MultiResult`, `MultiWarning`) and protocol constants (`AuthPrefix`, `TransitionAdd`/`Edit`/`Delete`).
- **`utils.go`** — Time formatting/parsing (`FormatISO8601`, `ParseISO8601`, `FormatDate`, `ParseDate`).

## Key Patterns

- **`Record` is `map[string]any`** with typed accessors (`GetString`, `GetFloat`, `GetInt`). This is the primary type for raw API responses. Field names are snake_case and app-specific; callers must know them.
- **Unified mutation via `Multi`** — create, edit, and delete all go through `POST /tickets/multi`. The operation is determined by the `transition` field value in each record (`add` / `edit` / `delete`). `Multi` returns `[]MultiResult` and does not inspect per-record `Successful` — that is the caller's responsibility.
- **Three list URL shapes** — choose based on what the caller already knows (see table below). `ListRecords` requires `publishing_alias`/`project_alias` in the rlx filter; `ListRecordsInApp` pushes workspace + app into the path; `ListRecordsByAppID` targets an app directly by ID.
- **`ListOptions.UsePOST`** — when true, the three list methods switch to `POST` with `application/x-www-form-urlencoded`, with all params in the body. Use this when the rlx filter is long enough to approach URL-length limits (~8 KB).
- **`ListOptions.Extra url.Values`** — arbitrary extra query params for forward-compat with API additions; keys overwrite named fields on conflict. Use sparingly.
- **`ListOptions.SkipAncestors` / `IncludeDeletedAfter`** — protocol flags on the listing endpoints. `SkipAncestors` trims ancestor records from the result; `IncludeDeletedAfter` is needed for incremental-sync flows so deletions appear alongside live records.
- **Incremental sync (`CountChangedInApp` / `CountChangedByAppID`)** — GET against `/apialpha.ashx/{scope}/tickets/changed?sinceTime=<ISO8601>`. Returns a count only; caller follows up with a `ListRecords*` call (commonly with `IncludeDeletedAfter=sinceTime`) to fetch the actual records. The server errors if extra params like `checksum`/`ts`/`layout_media` are included.
- **Session bootstrap (`GetCommon`)** — GET against `/apialpha.ashx/common`. Returns a `Record` with `user_name`, `tz`, `available_locales`, `system_configs`, `translations`, etc.
- **TUS file upload** — `UploadFile(r, size)` runs the TUS create → patch handshake in one call and returns the raw `Location` header value. Callers use that value as both `file_uid` and `id` inside an `attachments` entry when creating a record via `Multi`. Single PATCH only (no chunking/resumption).
- **Auth** — Custom `CMW_AUTH_CODE <token>` authorization header (not Bearer). Set via `client.SetAuthToken(token)`.
- **Logging** — Uses `log/slog` for HTTP error bodies. These are logged, not returned as structured errors. Tests that exercise non-2xx paths produce expected `ERROR` lines.
- **`http.Client` is injectable** — `NewClient(baseURL, httpClient)` accepts a custom client; pass `nil` for the default `&http.Client{}`. Tests use `httptest.NewServer` + `srv.Client()`.

## API URL Patterns

| Pattern | Usage |
|---|---|
| `{base}/tickets/list?listOfFields=&limitRecords=&rlx=&sortby=&skipAncestors=&includeDeletedAfter=` | Global record listing (GET, or POST-form when `UsePOST=true`) |
| `{base}/w/{wsAlias}/a/{appAlias}/tickets/list?...` | Workspace- and app-scoped listing |
| `{base}/aid/{appID}/tickets/list?...` | App-ID-scoped listing |
| `{base}/tickets/multi` | Bulk create / edit / delete (POST) |
| `{base}/upload-tus` (+ PATCH to Location) | TUS 1.0.0 file upload |
| `{base}/apialpha.ashx/common` | Session / user / config bootstrap (GET) |
| `{base}/apialpha.ashx/w/{wsAlias}/a/{appAlias}/tickets/changed?sinceTime=` | Workspace+app-scoped change count (GET) |
| `{base}/apialpha.ashx/aid/{appID}/tickets/changed?sinceTime=` | App-ID-scoped change count (GET) |

`baseURL` is expected to include the `/api` prefix (e.g. `https://extranet.example.com/api`).

## Time Formats

- ISO8601 with milliseconds: `2006-01-02T15:04:05.000Z` (timestamps)
- Date only: `2006-01-02` (date fields)

## Testing

- Tests are colocated with source: `client_test.go`, `upload_test.go`, `models_test.go`, `utils_test.go`.
- AAA pattern with explicit `// Arrange` / `// Act` / `// Assert` comments; one logical assertion per test.
- HTTP behavior is exercised end-to-end via `httptest.NewServer` + `NewClient(srv.URL, srv.Client())` — no `http.RoundTripper` mocking. Tests use closures to capture request state from the handler.
- `testify/assert` for soft checks, `testify/require` when a later assertion depends on the value being well-formed.
