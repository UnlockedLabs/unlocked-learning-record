# unlocked-learning-record

Learning Record: the resident achievement journal, with the entries a
resident records about a program or activity and the in-progress draft of
one. It was a module inside unlocked-core until EN-208, and its routes answer
exactly as they did there.

Its pages are still in core's frontend. The ingress sends their API path
here (`/api/learning-record`) and everything else to core.

## Commands

One binary, one image:

| Command | What it does |
|---|---|
| `app serve` | The API, on `ADDR`. Waits until the database is migrated. |
| `app migrate` | Runs the migrations. Run it after core's migrations. |
| `app migrate-reset` | Local only (`APP_ENV=dev`): rolls back the migrations, so core's `make migrate-fresh` can reset its own. Run it before core's reset. |

It has no background work, so there is no worker.

## Code

Laid out like core's backend and unlocked-open-content:

| Package | What it holds |
|---|---|
| `cmd/app` | The commands above. |
| `internal/models` | `LearningRecordEntry`, and the rows of core's `facilities` and `user_account_history` it reads and writes. |
| `internal/database` | The connection, the migrations and every query, on `database.DB`. |
| `internal/handlers` | The routes, their login and feature checks, and metrics. |
| `internal/server` | The HTTP server: health check, version, graceful stop. |

## Run it locally

Core's `make dev` will run Learning Record next to core once core's removal
PR adds it to docker compose. To run it on its own you need Go 1.26 and
Docker, plus core's Postgres and Kratos and core itself.

```sh
make migrate  # migrations
make run      # API on :8080
make test     # go test -race
make lint     # golangci-lint, in Docker
make build    # the image, named after the repo
```

Settings, all from the environment:

| Variable | What it does |
|---|---|
| `ADDR` | Where the API listens. Default `:8080`. |
| `PATH_PREFIX` | Leave unset: the routes live at `/api/learning-record`, like core's. It only moves `/api/version` and `/api/healthcheck`. |
| `APP_DSN`, or `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Core's database, as core reads them. |
| `KRATOS_PUBLIC_URL` | Kratos's public API, to check the login session. |
| `APP_URL` | Core inside the cluster, such as `http://server:8080`: who a user is. |
| `APP_ENV` | `dev` for local runs: no HSTS header, and `migrate-reset` may run. |
| `LOG_LEVEL` | Default `info`. Logs are JSON, as core's are. |
| `APP_VERSION`, `GIT_COMMIT`, `GIT_BRANCH` | What `/api/version` reports. CI bakes them into the image. |

## How it works with core

- **Login:** each request's Kratos session is checked with
  `identity.SessionChecker` from unlocked-contracts, which asks core
  (`/internal/v1/claims`) who the user is. A missing session is core's 401, a
  missing or wrong CSRF token on a write core's 403.
- **Feature flag:** every route needs `learning_record`, which core still
  owns. The check reads the session's claims (`claims.HasFeature`), which
  core sends with the user's identity, `feature_access` included, as Open
  Content and Programs do. So a feature is on or off here exactly when it is
  in core, with no second call. A user without it gets core's 401
  `Feature not enabled`.
- **Access:** every route is open to any logged-in user with the feature,
  as in core. Each route works on the caller's own entries only.
- **Logs:** an admin's write gets core's admin audit line
  (`"audit": "admin_action"`), with the same fields.
- **Metrics:** `/api/metrics` serves core's request metrics for Prometheus,
  labeled by route. The ingress never routes it.

## Database

Learning Record's table lives in core's database, in the `learning_record`
schema. Its queries run as the `learning_record` role: the connection
switches with `SET ROLE learning_record` and
`search_path = learning_record, public`, so the service logs in with core's
database user and needs no password of its own. The role can do only this:

- read and write `learning_record.learning_record_entries`;
- read core's `facilities`, for the achievement-location picker and an
  entry's `facility_name`;
- insert into core's `user_account_history` (and use its id sequence): when a
  resident deletes an entry, the delete and a `learning_record_deleted` row
  commit in one transaction.

Postgres refuses anything else.

- **Migrations** are in `internal/database/migrations` and run as core's
  database user. Their ledger is `public.goose_db_version_learningrecord`,
  the one core used, and the first migration is byte for byte core's, so a
  site that ran it inside core doesn't run it again. On a new database,
  core's migrations 00069 and 00073 still create the table in `public`, and
  this service's first migration moves it, so run it after core's. Never edit
  a merged migration; add a new one.
- **Create new tables as `learning_record.<name>`.** Migrations run as core's
  user, so an unqualified table lands in `public`, where the role can't use
  it.

## Tests

- `internal/handlers/routes_test.go` pins every route's status, JSON and
  feature flag against the golden files in
  `internal/handlers/testdata/learning_record`, which came unchanged from
  core. After an intended change, regenerate them and review the diff:

  ```sh
  go test ./internal/handlers -run 'TestLearningRecord' -update
  ```

- `account_history.golden` is core's `GET /api/users/{id}/account-history`
  answer after an entry delete. That route stays in core, so the test runs
  core's query on the row this service wrote.
- `PUT /api/learning-record/draft` (the upsert) is pinned as a 500: its
  `ON CONFLICT (user_id, client_id)` target doesn't match core's migration
  00069's partial unique index on `(user_id, client_id) WHERE deleted_at IS
  NULL`, and Postgres won't infer a partial index for a target that doesn't
  repeat its predicate (`42P10`). It's a production bug that predates the
  move and moved with it unchanged. The fix is
  `TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "deleted_at IS NULL"}}}`
  on the `clause.OnConflict` in `UpsertDraft`, plus `CURRENT_TIMESTAMP` for
  `NOW()` so the SQLite test can run it, in
  [EN-287](https://unlockedlabs.atlassian.net/browse/EN-287).
- The tests run on SQLite, and give the caller's claims directly (with or
  without `learning_record` in `FeatureAccess`), so they can't run the
  migrations or the role's grants. Check those against core's
  `make dev`.

## How to release

1. Merge PRs to `main` with conventional-commit titles (`feat: ...`,
   `fix: ...`). Each merge pushes `:sha-<short>` to ECR, then moves `:latest`
   to it if it's still the tip of `main`.
2. release-please keeps a `chore(main): release X.Y.Z` PR open with the
   changelog. A `feat` bumps the minor version, a `fix` the patch (before
   1.0.0, a breaking change bumps the minor version).
3. Merge that PR when you want a release. It tags `vX.Y.Z`, and CI pushes
   `:X.Y.Z` to ECR.

The `conventional` and `pr title` checks allow `feat`, `fix`, `perf`,
`revert`, `chore`, `docs`, `style`, `refactor`, `test`, `build` and `ci`. A
`!` after the type marks a breaking change.

## How to deploy

The templates live in `unlocked-platform` (`components.learningRecord`,
added in its EN-208 PR): the `serve` Deployment, with its readiness probe on
`/api/healthcheck`, and Service; a migrate Job that runs `app migrate` after
core's; and the `/api/learning-record` ingress path. Pin the release in each
site file and deploy through the deploy workflow. Only staging may use
`latest`.

## Vulnerability alerts

`govulncheck` checks the code, its dependencies and the Go version on every
push, every Monday at 14:00 UTC, and by hand (**Actions → govulncheck → Run
workflow**). A finding fails the check. When it fails on `main` and the repo
can see the `SLACK_WEBHOOK_URL` secret, it posts a link to the run to Slack.
Without the secret, it shows only as a red check.
