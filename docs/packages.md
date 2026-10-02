# Package reference

Import path prefix: `github.com/retailcrm/mg-transport-core/v2`. API reference for every package is on
[pkg.go.dev]; this page is a map of what exists and when to reach for it.

| Package | Purpose |
|---|---|
| [`core`](#core) | Engine, localizer, templates, Sentry, validator, job manager. |
| [`core/config`](#coreconfig) | YAML configuration loading and typed accessors. |
| [`core/db`](#coredb) | GORM/PostgreSQL ORM, migrations, models, migration generator. |
| [`core/healthcheck`](#corehealthcheck) | Per-account success-rate counters and CRM notifications. |
| [`core/logger`](#corelogger) | Structured logging with transport-specific scoping and adapters. |
| [`core/middleware`](#coremiddleware) | CSRF, Sentry injection, one-step-connection gin middlewares. |
| [`core/nats`](#corenats) | Shared NATS/JetStream client. |
| [`core/queue`](#corequeue) | Typed queues, worker pools, multi-queue store. |
| [`core/stacktrace`](#corestacktrace) | Stack traces attached to errors. |
| [`core/util`](#coreutil) | API client bootstrap, S3 avatars, phones, JSON helpers. |
| [`core/util/errorutil`](#coreutilerrorutil) | HTTP error responses and error collectors. |
| [`core/util/httputil`](#coreutilhttputil) | HTTP client builder with proxy and TLS knobs. |
| [`core/cache`](#corecache) | Typed cache facade and codecs. |
| [`cmd/transport-core-tool`](#cmdtransport-core-tool) | CLI helper (migration scaffolding). |

Dedicated guides: [engine](engine.md), [queues](queues.md), [cache](cache.md), [NATS](nats.md).

## core

The application engine and its direct collaborators.

- `New(AppInfo) *Engine` — composition root; see [Engine & web application](engine.md).
- `Engine`: `Prepare()`, `Run()`, `Serve(l)`, `Shutdown(ctx)`, `Router()`, `ConfigureRouter(cb)`,
  `CreateRenderer`/`CreateRendererFS` (templates), `WithCookieSessions`/`WithFilesystemSessions`,
  `InitCSRF` + `VerifyCSRFMiddleware`/`GenerateCSRFMiddleware`/`GetCSRFToken`, `JobManager()`,
  `Logger()`/`SetLogger`, `BuildHTTPClient`/`HTTPClient`, `UseZabbix`, `HijackGinLogs`.
- `GetApp(c)` / `MustGetApp(c)` — fetch the engine from a gin context.
- `Localizer` / `NewLocalizer` / `NewLocalizerFS` — i18n: per-request clones, `trans`/`transTpl`
  template functions, localized HTTP error payloads, `Accept-Language` matching.
- `Renderer` — multitemplate wrapper with filesystem and embedded-FS support.
- `Sentry` — SDK init, tagged context-aware captures (`CaptureException(c, err)`), panic-recovery
  middlewares, `NewTaggedStruct`/`NewTaggedScalar` for custom event tags.
- `JobManager` — named background jobs with intervals, panic isolation, sequential one-shot runs.
- Validator: the `validateCrmURL` binding rule (auto-registered) validates RetailCRM API URLs against
  known SaaS/box domains; used via the `binding:"validateCrmURL"` struct tag.
- `domains.go` — SaaS/box domain lists used by the validator.

## core/config

- `NewConfig(path)` — load YAML into `*Config` (panics on failure); `LoadConfigFromData` for bytes.
- `Config` + `Configuration` interface — typed accessors: `GetHTTPConfig`, `GetDBConfig`,
  `GetSentryDSN`, `GetAWSConfig`, `GetZabbixConfig`, `GetLogFormat`, `IsDebug`, `GetVersion`, ...
- `HTTPClientConfig` — timeout, SSL verification, proxying (including per-host split tunnel).

Transports with custom configuration embed the relevant fields and implement `Configuration`.

## core/db

- `ORM` / `NewORM(DatabaseConfig)` — GORM (PostgreSQL) with pool tuning from config.
- `Migrations()` — global registry; migrations self-register in `init()` via `Add`; apply with
  `SetDB(db).Migrate()`; also `Rollback`, `MigrateTo`, relative navigation, `Current`.
- `NewMigrationCommand` — the `migration` go-flags command backing the CLI tool.
- `ExecStatements(db, statements)` — run raw SQL sequentially (useful inside migrations).
- `models` — shared GORM models: `User`, `Connection` (with `Accounts` relation), `Account`.

## core/healthcheck

- `Counter` / `NewAtomicCounter(name)` — lock-free success/failure accounting with failed-state
  tracking; auto-resets every 15 minutes.
- `NewSyncMapStorage(constructor)` — counters keyed by account ID; `Get`, `Remove`, `Process`.
- `CounterProcessor` — default policy: alert when success ratio < 0.8 with ≥ 10 requests;
  deduplicates alerts; localizes messages.
- `DefaultNotifyFunc` — delivers notifications to CRM superadmins; `ConnectionDataProvider` resolves
  account ID to API credentials and language.

See [Engine & web application: health monitoring](engine.md#health-monitoring-corehealthcheck).

## core/logger

- `Logger` interface — zap-style levels plus `With`/`WithLazy`, `ForHandler`/`ForConnection`/
  `ForAccount` scoping, `Sync`.
- `NewDefault(format, debug)` — `"json"` or `"console"`; `NewNil()` silences output.
- `GinMiddleware(log, skipPaths...)` — per-request logging with generated `streamId`; skip paths
  support exact and wildcard matches; retrieve with `MustGet(c)`.
- Field helpers — `Err`, `Handler`, `StreamID`, `Body`, ... plus attribute-name constants for
  consistent keys (`handler`, `connection`, `account`, `streamId`).
- Adapters — `APIClientAdapter`, `MGTransportClientAdapter`, `ZabbixCollectorAdapter`,
  `WriterAdapter(log, level)`.

## core/middleware

- `NewCSRF(...)`, `GenerateCSRFMiddleware`/`VerifyCSRFMiddleware`, `CSRFErrorReason` — session-backed
  CSRF protection with typed failure reasons; usually driven through the Engine helpers.
- `InjectSentry` / `CaptureException(c, err)` etc. — Sentry captures from gin handlers without a
  direct dependency.
- `ConnectionConfig(url, scopes)`, `VerifyConnectRequest(secret)`, `MustGetConnectRequest(c)` —
  one-step-connection flow for RetailCRM integrations.

## core/nats

- `Connect(ctx, Config, log, extraOpts...) (*Client, error)` — one shared connection with defaults,
  single-method auth, TLS, and event logging.
- `Client`: `Conn` (*nats.Conn), `JetStream` context, `Drain(ctx)`, `Close()`.

See [NATS integration](nats.md).

## core/queue

- `NewStore(driverFor, processor, policy, opts...)` — one executor per queue ID; `Enqueue`, `Get`,
  `Info`, `Has`, `Reconcile`, `Remove`, `Stats`, `Drain`, `CloseIntake`, `Stop`.
- `JobEnvelope[T]` — `Value`, `Metadata`, `Ack`, `Requeue`, `Reject`, `Touch`, `Settled`.
- `AutoRenewableJobEnvelope` — optional `AutoRenewInterval` for automatic lease renewal.
- `DeadLetterJobEnvelope` — optional `DeadLetter`; use `queue.DeadLetter` to preserve terminal failures.
- `ErrJobEnvelopeSettled` — returned when an envelope is settled more than once.
- `WorkerPolicy` — scaling bounds and timing; `DesiredWorkersFunc` for custom scaling.
- Options: `WithID`, `WithDelay`, `WithNotBefore`; store options `WithPanicHandler`,
  `WithUnsettledProcessor`, `WithWorkerFactory`.
- Codecs: `JSONCodec[T]`, `BytesCodec`, `FuncCodec[T]` (hydrate runtime dependencies on decode).
- Drivers: [`queue/memory`](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/v2/core/queue/memory)
  (process-local), [`queue/beanstalk`](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/v2/core/queue/beanstalk)
  (beanstalkd, auto-reconnecting `Manager`), [`queue/nats`](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/v2/core/queue/nats)
  (JetStream stream + durable consumer + message schedules).

See [Queues](queues.md).

## core/stacktrace

- `AppendToError(err, skip...)` — attach a stack trace to an error unless it already has one.
- `FormattedStack(skip, prefix)` — ready-to-log stack dump with source snippets.

Pass traced errors to `Sentry.CaptureException` for meaningful reports.

## core/util

- `Utils` / `NewUtils` — `GenerateToken`, `GetAPIClient(url, key, scopes)` (RetailCRM API client with
  scope enforcement), `UploadUserAvatar(url)` (S3 upload), `ResetUtils`.
- Phone handling — `ParsePhone(number)` with country-specific corrections,
  `FormatNumberForWA(number)` (E.164 per WhatsApp rules).
- Misc — `GetMGItemData`, `GetEntitySHA1`, `BindJSONWithRaw`, currency symbol helpers.

## core/util/errorutil

- `BadRequest`/`Unauthorized`/`Forbidden`/`InternalServerError` — `(status, response)` pairs.
- `Collector` — accumulate errors with call sites; `AsError`, `Iterate`, `Panic`.
- `NewInsufficientScopesErr` / `ErrInsufficientScopes` — typed scope errors compatible with
  `errors.Is`.

## core/util/httputil

- `NewHTTPClientBuilder()` — fluent builder: timeout, SSL verification, TLS 1.0, proxy with
  per-host/CIDR split tunnel, cert pool, `FromConfig(*config.HTTPClientConfig)`.
- `ReplaceDefault()`/`RestoreDefault()` — swap the global default client when a dependency cannot be
  parameterized.

## core/cache

- `New(driver) *Cache[K, V]` — typed facade: `Get`, `Set`, `Has`, `Delete`, `Clear`, `Len`, `Close`.
- `Driver[K, V]` interface — implemented by
  [`cache/memory`](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/v2/core/cache/memory)
  (otter, capacity + write TTL) and
  [`cache/nats`](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/v2/core/cache/nats)
  (JetStream KV bucket, server-side TTL, shared across replicas).
- Converters: `JSONCodec[T]`, `BytesCodec`, `StringKeyEncoder`, `JSONKeyEncoder[K]`.

See [Cache](cache.md).

## cmd/transport-core-tool

CLI helper installed with `go install ./cmd/transport-core-tool`. Currently one command:

```sh
transport-core-tool migration [-d ./migrations]
```

It scaffolds `<unix-timestamp>_app.go` containing a `gormigrate.Migration` skeleton wired to
`db.Migrations().Add(...)`; apply with `db.Migrations().SetDB(db).Migrate()`.

[pkg.go.dev]: https://pkg.go.dev/github.com/retailcrm/mg-transport-core/v2
