# TaskFlow

TaskFlow is a collaborative project-management REST API written in Go. It supports authenticated project workspaces, task planning, project members, activity history, and AI-assisted project-risk analysis powered by Groq.

## Features

- Cookie-based authentication with email verification, password hashing, and session logout.
- Projects with owner, editor, and viewer access roles.
- Tasks with assignees, priorities, due dates, estimates, acceptance criteria, state changes, and optimistic locking.
- Task dependencies with duplicate and cycle prevention.
- Filtering, sorting, and paginated project, task, activity, and AI-run lists.
- Immutable activity history for important mutations.
- Background assignment notifications through SMTP/Mailpit.
- Per-client rate limiting, secure JSON decoding, server timeouts, panic recovery, and graceful shutdown.
- Queued AI analysis jobs processed by a Go worker using Groq structured output.

## Architecture

```text
HTTP request
    -> middleware (security headers, logging, authentication, rate limiting)
    -> handler
    -> model
    -> PostgreSQL

AI analysis request
    -> durable ai_analysis_runs record (queued)
    -> background worker claims the run with SKIP LOCKED
    -> Groq structured-output request
    -> succeeded or failed run persisted for later retrieval
```

The application uses the Go standard library's `net/http` router, `database/sql` with PostgreSQL, and SQL migrations. Application code is under `cmd/api` and reusable internal packages live under `internal`.

## Prerequisites

- Go 1.26+
- Docker Desktop and Docker Compose
- PostgreSQL migration CLI: [golang-migrate](https://github.com/golang-migrate/migrate)
- A Groq API key for live AI analysis

## Local setup

1. Create your local environment file.

   ```powershell
   Copy-Item .env.example .env
   ```

   Set a strong `POSTGRES_PASSWORD`, then use that same value in `DATABASE_URL` and `TEST_DATABASE_URL`. Set `GROQ_API_KEY` locally. Never commit `.env`.

2. Start PostgreSQL and Mailpit.

   ```powershell
   docker compose up -d --wait
   ```

   Mailpit is available at <http://localhost:8025>.

3. Create the test database once.

   ```powershell
   docker compose exec db psql -U taskflow -d postgres -c "CREATE DATABASE taskflow_test;"
   ```

4. Apply migrations to both databases.

   ```powershell
   $databaseLine = Get-Content .env | Where-Object { $_ -like 'DATABASE_URL=*' } | Select-Object -First 1
   $env:DATABASE_URL = (($databaseLine -split '=', 2)[1]).Trim()

   $testDatabaseLine = Get-Content .env | Where-Object { $_ -like 'TEST_DATABASE_URL=*' } | Select-Object -First 1
   $env:TEST_DATABASE_URL = (($testDatabaseLine -split '=', 2)[1]).Trim()

   migrate -path ./migrations -database $env:DATABASE_URL up
   migrate -path ./migrations -database $env:TEST_DATABASE_URL up
   ```

5. Run the API.

   ```powershell
   go run ./cmd/api
   ```

   The API listens on `http://localhost:8081` by default. Check it with:

   ```powershell
   curl.exe http://localhost:8081/health
   ```

## Tests

Load the test database URL and run all tests:

```powershell
$testDatabaseLine = Get-Content .env | Where-Object { $_ -like 'TEST_DATABASE_URL=*' } | Select-Object -First 1
$env:TEST_DATABASE_URL = (($testDatabaseLine -split '=', 2)[1]).Trim()

go test ./... -count=1
go vet ./...
```

The real Groq check is deliberately opt-in so normal tests never consume provider usage:

```powershell
$groqKeyLine = Get-Content .env | Where-Object { $_ -like 'GROQ_API_KEY=*' } | Select-Object -First 1
$env:GROQ_API_KEY = (($groqKeyLine -split '=', 2)[1]).Trim()
$env:RUN_GROQ_LIVE_TEST = "1"

go test ./internal/ai -run '^TestGroqAnalyzerLive$' -count=1
```

## API overview

All routes except registration, login, email verification, health, and greeting require the session cookie created by `POST /sessions`.

| Area | Endpoints |
| --- | --- |
| Health | `GET /health` |
| Users and sessions | `POST /users`, `POST /users/verify-email`, `POST /sessions`, `DELETE /sessions/current`, `GET /me` |
| Projects | `POST/GET /projects`, `GET/PATCH/DELETE /projects/{id}` |
| Members | `POST/GET /projects/{id}/members`, `PATCH/DELETE /projects/{id}/members/{userID}` |
| Tasks | `POST/GET /projects/{id}/tasks`, `GET/PATCH/DELETE /projects/{id}/tasks/{taskID}` |
| Dependencies | `POST/GET /projects/{id}/tasks/{taskID}/dependencies`, `DELETE /projects/{id}/tasks/{taskID}/dependencies/{dependsOnTaskID}` |
| Activity | `GET /projects/{id}/activity` |
| AI analysis | `POST /projects/{id}/analysis-runs`, `GET /projects/{id}/analysis-runs`, `GET /projects/{id}/analysis-runs/{runID}` |

`POST /projects/{id}/analysis-runs` returns `202 Accepted`. The background worker processes the saved project snapshot and the client can poll the returned run URL for `queued`, `running`, `succeeded`, or `failed` status.

## Development status

The backend core is complete. Future work can include deployment, OpenAPI documentation, a frontend, observability, additional worker retry policies, and human-approved AI actions.
