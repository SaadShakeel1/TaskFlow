**# TaskFlow: Next Session Handoff**

**## Project goal**

TaskFlow is a learning-focused, production-style collaborative project-management REST API built in Go. The learner wants to understand and write the code incrementally, using the Go standard library (\`net/http\`) before frameworks.

**## Mentoring preferences**

\- Treat the learner as a beginner when explaining new code.

\- Before suggesting a terminal command, explain what it does, its important options, and whether it changes files or Git state.

\- Provide code blocks and clear VS Code navigation when asking the learner to edit files.

\- Keep changes small. Explain first, give a small task, review it, then continue.

\- Do not generate the entire application or introduce abstractions/folders prematurely.

\- Do not perform destructive Git operations, commit, push, merge, or delete branches without the learner explicitly asking.

\- Use realistic Git workflow, but do not create unnecessary branches for trivial work.

**## Current repository state**

\- Module: \`github.com/SaadShakeel1/TaskFlow\`

\- Go installed: \`go1.26.4 windows/amd64\`

\- Remote: \`origin\` -> \`https\://github.com/SaadShakeel1/TaskFlow\.git\`

\- Local branch currently checked out: \`main\`

\- \`main\` and \`origin/main\` currently contain only:

  \`\`\`text

  75a568d chore: initialize project

  \`\`\`

\- The feature branch has been pushed:

  \`\`\`text

  feat/health-endpoint

  88cb4f5 feat: add health check endpoint

  \`\`\`

\- Based on local remote-tracking refs, the feature is not merged into \`main\` yet.

**## Completed work**

\- Initialized Git with \`main\` as the initial branch.

\- Added \`.gitignore\`, \`README.md\`, and \`go.mod\`.

\- Created and committed the project baseline:

  \`\`\`text

  chore: initialize project

  \`\`\`

\- Created \`feat/health-endpoint\`.

\- Implemented a basic Go HTTP server and \`GET /health\` endpoint.

\- Added an automated endpoint test with \`net/http/httptest\`.

\- Committed and pushed the health feature:

  \`\`\`text

  feat: add health check endpoint

  \`\`\`

**## Health endpoint implementation**

The feature branch adds these files:

\`\`\`text

main.go

main\_test.go

\`\`\`

\`main.go\` contains:

\- \`main()\`, which starts an HTTP server.

\- \`newMux()\`, which creates the router and registers \`GET /health\`.

\- \`healthHandler\`, which returns \`200 OK\` and JSON equivalent to:

  \`\`\`json

  {"status":"ok"}

  \`\`\`

\`main\_test.go\` contains \`TestHealthEndpoint\`. It creates a fake HTTP request/response with \`httptest\`, passes the request through \`newMux()\`, then verifies:

\- status is \`200 OK\`;

\- \`Content-Type\` is \`application/json\`;

\- decoded JSON has \`status: "ok"\`.

**## Local port note**

Port \`8080\` is used by a local Oracle listener process (\`TNSLSNR\`) and returned \`401 Unauthorized\` when queried. Do not stop that process automatically.

TaskFlow therefore listens on port \`8081\` for now:

\`\`\`text

http\://localhost:8081/health

\`\`\`

The hardcoded port is acceptable for this first milestone. Introduce configuration/environment variables later, when there is a real need.

**## Useful local commands**

Run the application:

\`\`\`powershell

go run .

\`\`\`

Test the health endpoint from a second terminal:

\`\`\`powershell

curl.exe -i http\://localhost:8081/health

\`\`\`

Run automated tests:

\`\`\`powershell

go test .

\`\`\`

Format Go files:

\`\`\`powershell

go fmt .

\`\`\`

**## Next step: complete the Pull Request workflow**

1\. On GitHub, open the TaskFlow repository.

2\. Create a Pull Request with:

   \`\`\`text

   base: main

   compare: feat/health-endpoint

   \`\`\`

3\. Suggested PR title:

   \`\`\`text

   feat: add health check endpoint

   \`\`\`

4\. Suggested PR description:

   \`\`\`markdown

   **## What changed**

   - Added a Go HTTP server.

   - Added \`GET /health\`, returning a JSON health response.

   - Added an automated endpoint test.

   **## Why**

   Establishes the first working HTTP endpoint and test setup for TaskFlow.

   **## Testing**

   - \`go test .\`

   - Manually requested \`GET /health\`

   \`\`\`

5\. Review the **\*\*Files changed\*\*** tab.

6\. When the learner chooses to merge, use **\*\*Create a merge commit\*\*** for this early learning workflow.

7\. After GitHub merges the PR, synchronize locally:

   \`\`\`powershell

   git switch main

   git pull --ff-only

   \`\`\`

8\. Confirm the merged history with:

   \`\`\`powershell

   git log --oneline -3

   \`\`\`

Do not delete \`feat/health-endpoint\` until the learner explicitly wants to practice branch cleanup.

**## Complete development roadmap

This is a learning roadmap, not a requirement to build every possible feature. Each phase should be split into small, reviewable feature branches and should introduce only the concepts needed at that time.

The core learning target is broad backend engineering: Go, HTTP, PostgreSQL, authentication, authorization, relational modeling, business rules, testing, background work, graceful shutdown, Docker, and professional Git/GitHub workflow.

### Phase 0: GitHub workflow and health endpoint (current)

**Goal:** Establish a stable repository, a minimal HTTP server, a health endpoint, an automated test, and the first Pull Request workflow.

**Completed:**

- Git repository, Go module, `.gitignore`, and README.
- `GET /health` returning JSON.
- `httptest` coverage for the endpoint.
- Local `main`, remote `main`, and pushed `feat/health-endpoint` branch.

**Remaining:**

- Open, review, and merge the health-endpoint Pull Request.
- Sync local `main` after the merge.
- Practice branch cleanup only when the learner is ready.

### Phase 1: HTTP and API foundations

**Goal:** Become comfortable building small HTTP endpoints with the standard library before adding a database or framework.

**Topics:**

- `http.ServeMux` patterns, methods, path parameters, and query parameters.
- Named handlers and request/response structs.
- JSON decoding and encoding.
- HTTP status codes and headers.
- Input validation.
- Consistent API errors.
- `httptest` handler tests.
- Request flow: mux -> handler -> response.

Keep this phase small. A temporary/simple domain endpoint can be used for HTTP practice, but do not build the old project/task-management domain.

**Done when:** the learner can independently explain and implement a basic JSON handler and its test.

### Phase 2: PostgreSQL and persistence

**Goal:** Introduce durable data and relational modeling.

**Topics:**

- PostgreSQL setup.
- SQL migrations and migration ordering.
- `database/sql`.
- Connection pools.
- `context.Context` in database calls.
- Primary keys, foreign keys, constraints, and indexes.
- Transactions.
- Integration-test strategy.
- Repository abstraction only when direct database code becomes difficult to manage.

The first persisted resource will likely be `users`, followed by organizations/teams as the domain grows.

**Done when:** one real resource can be created and retrieved from PostgreSQL through tested Go code.

### Phase 3: Users and authentication

**Goal:** Securely identify users.

**Features:**

- User registration.
- Login.
- Refresh tokens.
- Protected endpoints.

**Topics:**

- Request validation.
- Duplicate-email handling.
- Password hashing.
- JWT access tokens.
- Refresh-token lifecycle.
- Authentication middleware.
- Request-scoped authenticated identity.
- Authentication vs authorization.
- Secrets/configuration.

**Done when:** protected requests have an authenticated user identity available through the request context.

### Phase 4: Organizations, teams, and membership

**Goal:** Build the collaboration/ownership foundation without turning TaskFlow into a generic project manager.

Domain:

```text
Organization
  -> Members
  -> Teams
  -> Team Members
```

Possible roles:

```text
ORG_OWNER
TEAM_ADMIN
RESPONDER
VIEWER
```

Exact permissions should be introduced only when endpoints require them.

**Topics:**

- One-to-many and many-to-many relationships.
- Membership tables.
- Resource ownership.
- RBAC.
- Authorization middleware/service checks.
- Allowed/denied authorization tests.
- Transactions where membership creation requires multiple writes.

**Done when:** users can access organization/team resources only according to their membership and role.

### Phase 5: Services

**Goal:** Let engineering teams register the systems they operate.

A service can initially contain:

```text
ID
Name
Description
OwningTeamID
Criticality
CreatedAt
UpdatedAt
```

Potential criticality values:

```text
LOW
MEDIUM
HIGH
CRITICAL
```

**Features:**

- Create a service.
- List services for a team/organization.
- View a service.
- Update service metadata.
- Archive/delete according to business rules.

**Topics:**

- Nested/resource-scoped APIs.
- Authorization based on team ownership.
- Validation.
- Filtering and pagination.
- Database indexes guided by query patterns.

### Phase 6: Incidents - core domain

**Goal:** Build the central feature of TaskFlow.

An incident can contain:

```text
Title
Description
AffectedService
Severity
Status
IncidentCommander
StartedAt
ResolvedAt
```

Possible severities:

```text
SEV1
SEV2
SEV3
SEV4
```

Possible lifecycle:

```text
DETECTED
  -> INVESTIGATING
  -> MITIGATED
  -> RESOLVED
  -> POSTMORTEM_PENDING
  -> CLOSED
```

Do not treat status as unrestricted CRUD. Define valid state transitions and reject invalid transitions.

**Topics:**

- Business/domain rules.
- State transitions/state-machine thinking.
- Validation beyond request syntax.
- Transactions.
- Domain-specific errors.
- Filtering incidents by service, severity, status, and time.
- Pagination and sorting.
- Business-rule tests.

**Done when:** incidents have a tested lifecycle rather than being arbitrary database rows.

### Phase 7: Responders and incident coordination

**Goal:** Model the people actively working an incident.

**Features:**

- Assign an incident commander.
- Add/remove responders.
- List responders.
- Restrict incident updates to appropriately authorized users.

Example:

```text
Incident: Checkout API timing out
Severity: SEV1

Commander: Saad
Responders:
- Ali
- Ahmed
```

**Topics:**

- Many-to-many relationships.
- Role/resource-specific authorization.
- Duplicate membership prevention.
- Transactional updates.
- Concurrency considerations when multiple users modify an incident.

### Phase 8: Incident timeline / audit trail

**Goal:** Preserve the history of what happened during an incident.

Example:

```text
10:02 Incident detected
10:05 Database latency identified
10:08 Rollback started
10:12 Error rate recovered
10:15 Incident mitigated
```

Some entries may be written manually; others can eventually be generated automatically from important domain changes.

**Topics:**

- Append-oriented data.
- Audit/event history.
- Ordering by timestamp.
- Transactions: e.g. status change + timeline event.
- Immutable-history thinking.
- Pagination for timelines.
- Testing side effects.

### Phase 9: Action items

**Goal:** Turn incident lessons into trackable follow-up work.

Action items can contain:

```text
Title
Description
IncidentID
AssigneeID
Priority
Status
DueDate
```

Example:

```text
- Add database timeout
- Add regression test
- Improve alert threshold
```

Keep these scoped to incident remediation rather than evolving back into a generic Jira clone.

**Topics:**

- Relationships.
- Assignment authorization.
- Due dates and time handling.
- Status/priority validation.
- Filtering, sorting, and pagination.
- Overdue queries.
- SQL indexes.

### Phase 10: Postmortems

**Goal:** Capture structured learning after an incident.

A postmortem can contain:

- Summary.
- Impact.
- Root cause.
- Detection.
- Resolution.
- Lessons learned.
- Preventive actions.

Business rules might include:

- A postmortem belongs to an incident.
- It normally becomes relevant after resolution.
- Closing an incident may require a postmortem for high-severity incidents.

Do not implement rules blindly; decide them explicitly when this phase begins.

**Topics:**

- One-to-one relationships.
- Cross-resource business rules.
- Transactions.
- Validation.
- Authorization.
- More substantial service/business-layer testing.

### Phase 11: Notifications and background jobs

**Goal:** Use Go concurrency for a real application need.

Possible events:

- User added as incident responder.
- Incident severity escalated.
- Incident resolved.
- Action item approaching/overdue.

Initial architecture:

```text
Incident/action event
        -> notification job
        -> buffered jobs channel
        -> worker pool
        -> notification processing
```

The first implementation can persist/log notifications rather than integrating a real email provider.

**Topics:**

- Goroutines.
- Channels.
- Buffered vs unbuffered channels.
- Worker pools.
- `select`.
- `sync.WaitGroup`.
- Context cancellation.
- Backpressure.
- Error handling in background work.
- Graceful worker shutdown.
- Race detector.

**Important:** understand the synchronous implementation first. Do not add goroutines merely to demonstrate concurrency.

### Phase 12: Testing maturity

Testing begins in Phase 0 and continues throughout the project. This phase is about filling important gaps rather than suddenly adding all tests at the end.

**Topics:**

- Table-driven tests.
- `httptest`.
- Business-rule/service tests.
- Authorization tests.
- Database integration tests.
- Fakes/mocks only when useful.
- `go test ./...`.
- `go test -race ./...`.
- Coverage as a diagnostic, not a target by itself.

Important scenarios should include invalid incident transitions, unauthorized actions, duplicate responders, transaction failures, and background-worker shutdown.

### Phase 13: Production readiness

**Goal:** Make TaskFlow robust to run and operate.

Introduce incrementally:

- Configuration/environment variables.
- Structured logging.
- Request IDs.
- Panic recovery.
- Server read/write/idle timeouts.
- Graceful HTTP shutdown.
- OS signals.
- Shared root context.
- Database cleanup.
- Worker shutdown.
- Rate limiting where justified.
- CORS where justified.
- Health/readiness checks.

Graceful shutdown should eventually look conceptually like:

```text
SIGINT/SIGTERM
  -> cancel root context
  -> stop accepting HTTP requests
  -> stop producing background jobs
  -> finish/cancel workers safely
  -> close database resources
  -> exit
```

### Phase 14: Docker, CI, and documentation

**Goal:** Make the repository easy for another developer to run and contribute to.

**Topics/features:**

- Dockerfile.
- Multi-stage Go build.
- Docker Compose with PostgreSQL.
- Environment configuration.
- Database migrations in local setup.
- GitHub Actions for formatting/tests/build.
- README architecture and setup instructions.
- API documentation/OpenAPI.
- Contributor documentation when useful.

### Phase 15: Optional observability

Only after the core application is strong.

Possible additions:

- `/metrics`.
- Request counts/latency.
- Background-job metrics.
- Incident/notification operational metrics.
- Prometheus.
- Grafana.
- Basic tracing later if there is a learning reason.

Do not let optional infrastructure prevent v1 from being considered complete.

## Suggested v1 boundary

A strong v1 is complete when TaskFlow has:

```text
Users + Authentication
Organizations + Teams
Services
Incidents + Valid State Transitions
Responders
Incident Timeline
Action Items
Postmortems
RBAC/Authorization
PostgreSQL
Tests
Background Notification Workers
Graceful Shutdown
Docker
CI
Documentation
```

Redis, Kafka/RabbitMQ, WebSockets, Kubernetes, microservices, distributed tracing, and other infrastructure are **not required for v1**.

## Learning expectations

At roughly 3-4 focused hours per day, expect approximately **4-5 weeks (about 100-130 hours)** for a strong learning-oriented v1. This is not a deadline.

A day spent understanding SQL transactions, `context.Context`, JWT validation, authorization, or worker shutdown is productive even if little code is written.

The objective is not to maximize generated code. The objective is that the learner can explain the architecture and continue developing without depending on AI to generate entire features.

## Git workflow for future features**

For a meaningful feature or fix:

\`\`\`text

Issue (when useful)

  -> feature/fix branch

  -> small implementation and tests

  -> format and test

  -> inspect diff

  -> focused commit

  -> push branch

  -> Pull Request review

  -> merge into main

\`\`\`

Suggested branch prefixes:

\`\`\`text

feat/\<description>

fix/\<description>

test/\<description>

refactor/\<description>

docs/\<description>

\`\`\`

Use a separate branch when the work is a meaningful deliverable, bug fix, or refactor. Do not create a branch for every minor edit during a feature.

**## Architecture direction**

The current root-level layout is intentionally appropriate for one endpoint. Do not refactor only to create folders.

When additional endpoints and setup justify it, gradually consider a layout like:

\`\`\`text

cmd/api/main.go

internal/httpapi/

migrations/

\`\`\`

Do not introduce \`service\`, \`repository\`, database, authentication, Docker, or concurrency layers until the relevant feature naturally needs them.