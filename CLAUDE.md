# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go client library for the [ComindWork](https://comindwork.com) (Extranet) REST API. Zero external dependencies — stdlib only. Module: `github.com/Hukyl/comindwork-go`, requires Go 1.22+.

## Commands

```bash
go build ./...          # Build
go test ./...           # Run all tests
go test -run TestFoo    # Run a single test
go vet ./...            # Static analysis
```

No Makefile, no linter config, no CI pipeline exists yet.

## Architecture

Single-package library (`package comindwork`) with three files:

- **`client.go`** — `APIClient` struct and all public/private methods. Layers: unexported HTTP helpers (`get`, `post`, `applyAuth`, `isRespError`) → unexported URL builders (`listURL`, `scopedListURL`, `multiURL`) → public domain operations (records, users, workspaces, workdays, timelogs, session).
- **`models.go`** — All types (`Workspace`, `Workday`, `TimeLog`, `Task`, `User`, `Record`, `ListOptions`, `MultiResult`, `MultiWarning`) and constants (app aliases, app IDs, transitions).
- **`utils.go`** — Time formatting/parsing (`FormatISO8601`, `ParseISO8601`, `FormatDate`, `ParseDate`, `CalculateTotalReal`) and task reference parsing (`ParseTaskReference`, `FormatTaskReference`).

## Key Patterns

- **`Record` is `map[string]any`** with typed accessors (`GetString`, `GetFloat`, `GetInt`). This is the primary type for raw API responses.
- **Transition-based mutations** — Create, edit, and delete all go through `POST /tickets/multi`. The operation is determined by the `transition` field value (`add`, `edit`, `delete`).
- **Workspace cache** — `APIClient` holds an in-memory `map[string]*Workspace` behind a `sync.RWMutex`. Populated manually via `RegisterWorkspace`, not fetched from the API.
- **Auth** — Custom `CMW_AUTH_CODE <token>` authorization header (not Bearer). Set via `client.SetAuthToken(token)`.
- **Logging** — Uses `log/slog` for warnings on skippable parse failures and HTTP errors. These are logged, not returned as errors.
- **`http.Client` is not injectable** — hardcoded `&http.Client{}` in `NewClient`. Testing HTTP interactions requires refactoring to accept a custom client or `http.RoundTripper`.

## API URL Patterns

| Pattern | Usage |
|---|---|
| `{base}/tickets/list?listOfFields=&limitRecords=&rlx=&sortby=` | Global record listing |
| `{base}/w/{wsAlias}/a/{appAlias}/tickets/list?...` | Workspace+app scoped listing |
| `{base}/tickets/multi` | Bulk create/update/delete (POST) |
| `{base}/aid/{appID}/tickets/list?...` | App-ID-based listing (users) |
| `{base}/ping` | Session keepalive |

## Time Formats

- ISO8601 with milliseconds: `2006-01-02T15:04:05.000Z` (timestamps)
- Date only: `2006-01-02` (workday dates)
