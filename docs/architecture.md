# Architecture overview

This document explains how the library is organized and how the parts connect. Each subsystem has a
dedicated guide with usage examples — see the [documentation index](README.md).

## Layer map

The library is deliberately layered: the engine composes high-level services, while queues, caches,
and the NATS client are standalone subsystems a transport can use independently.

```mermaid
flowchart TB
    subgraph transport["Transport application"]
        APP["main.go<br/>(transport code)"]
    end

    subgraph engine["core (engine)"]
        E["Engine"]
        CFG["core/config"]
        LOC["Localizer"]
        TPL["Renderer / templates"]
        SEN["Sentry"]
        JM["JobManager"]
        MW["core/middleware"]
    end

    subgraph infra["Infrastructure subsystems"]
        Q["core/queue (+ drivers)"]
        C["core/cache (+ drivers)"]
        N["core/nats"]
        L["core/logger"]
        HC["core/healthcheck"]
        DB["core/db"]
        U["core/util"]
    end

    subgraph externals["External systems"]
        CRMR["RetailCRM API"]
        BS["beanstalkd"]
        NS["NATS / JetStream"]
        PG[(PostgreSQL)]
        SENTRY["Sentry"]
    end

    APP --> E
    E --> CFG
    E --> LOC
    E --> TPL
    E --> SEN
    E --> JM
    E --> MW
    E --> L
    E --> DB

    APP --> Q
    APP --> C
    Q --> N
    C --> N
    N --> NS
    Q --> BS
    DB --> PG
    SEN --> SENTRY
    U --> CRMR
    HC --> CRMR
```

## The engine composition

`core.Engine` is the composition root for the web-facing half of a transport. `Prepare()` wires the
logger, database, Sentry SDK, and localization; `Run()` serves the configured gin router.

```mermaid
flowchart LR
    NEW["core.New(AppInfo)"] --> CFGLOAD["app.Config = core.NewConfig(...)"]
    CFGLOAD --> ROUTER["app.ConfigureRouter(...)"]
    ROUTER --> PREP["app.Prepare()"]
    PREP --> RUN["app.Run()"]

    subgraph prepare["Inside Prepare()"]
        direction TB
        P1["Load translations<br/>& preload languages"]
        P2["Create DB (GORM/Postgres)"]
        P3["Create zap logger"]
        P4["Init Sentry SDK"]
    end

    subgraph runtime["Request path"]
        direction TB
        R1["gin router"] --> R2["Sentry middlewares"]
        R2 --> R3["Localization middleware"]
        R3 --> R4["Transport handlers<br/>(core.GetApp(c))"]
    end

    PREP -.-> prepare
    RUN -.-> runtime
```

Key properties:

- Handlers retrieve the engine from the gin context with `core.GetApp(c)` / `core.MustGetApp(c)` —
  the router injects it automatically.
- The localizer is cloned per request from the `Accept-Language` header; templates get `trans` /
  `transTpl` functions for free.
- Errors captured by the Sentry middlewares are tagged with connection/account IDs taken from the gin
  context, so production issues are traceable to a tenant.

See [Engine & web application](engine.md) for the full walkthrough.

## The queue pipeline

`core/queue` decouples *what* to process (typed items) from *where* they are stored (drivers) and
*how* they are consumed (autoscaling worker pools). One `Store` manages one executor per numeric queue
ID — typically a transport account ID.

```mermaid
flowchart TB
    subgraph store["Store[T]"]
        direction TB
        GET["Enqueue / Get / Reconcile"]
        EX1["Executor (id=1)"]
        EX2["Executor (id=2)"]
    end

    subgraph executor1["Executor internals"]
        direction TB
        Q1["Queue[T]"] --> B1["Driver[T]"]
        WG["workerGroup<br/>(WorkerPolicy)"] --> W1["Worker"]
        WG --> W2["Worker"]
        W1 --> P["Processor(ctx, id, JobEnvelope[T])"]
        W2 --> P
    end

    subgraph drivers["Driver implementations"]
        MEM["queue/memory<br/>process-local"]
        BS2["queue/beanstalk<br/>beanstalkd tube"]
        NB["queue/nats<br/>JetStream stream + consumer"]
    end

    GET --> EX1
    GET --> EX2
    EX1 --> Q1
    B1 --> MEM
    B1 -.-> BS2
    B1 -.-> NB
```

Job envelope lifecycle:

```mermaid
stateDiagram-v2
    [*] --> Enqueued: Enqueue(value)
    Enqueued --> Deferred: WithDelay / WithNotBefore
    Deferred --> Ready: due time reached
    Enqueued --> Ready
    Ready --> InFlight: Dequeue
    InFlight --> InFlight: Touch (renew lease)
    InFlight --> Ready: Requeue(delay) / lease expired
    InFlight --> Enqueued: Requeue(0)
    InFlight --> Done: Ack
    InFlight --> Done: Reject
    Done --> [*]
```

An envelope that reaches the processor is *settled* exactly once. If the processor returns or panics
without settling, the envelope stays pending in the driver and an optional
`WithUnsettledProcessor` hook observes it.

See [Queues](queues.md).

## The cache layers

`core/cache` is a typed facade (`Cache[K, V]`) over interchangeable drivers. In-memory drivers are
process-local and TTL-bounded; NATS drivers store entries in JetStream key-value buckets shared by
every replica of the transport.

```mermaid
flowchart LR
    CODE["transport code"] --> FACADE["cache.Cache[K, V]<br/>get/set/has/delete/clear/len"]
    FACADE --> BEH["cache.Driver[K, V]"]
    BEH --> MEMB["cache/memory<br/>otter cache<br/>capacity + write TTL"]
    BEH --> NATB["cache/nats<br/>JetStream KV bucket<br/>server-side TTL"]
    NATB --> KEYENC["KeyEncoder[K]<br/>(string or JSON+base64 keys)"]
    NATB --> VALCODEC["Codec[V]<br/>(JSON or raw bytes)"]
```

See [Cache](cache.md).

## The NATS stack

`core/nats.Client` owns a single NATS connection with JetStream enabled. Queue drivers (streams,
durable consumers, message schedules) and cache drivers (KV buckets) are built on the same client,
so a transport needs exactly one connection regardless of how many subsystems it uses.

```mermaid
flowchart TB
    subgraph client["core/nats.Client (one TCP connection)"]
        CONN["nats.Conn<br/>auth, TLS, reconnects"]
        JS["jetstream.JetStream"]
        CONN --> JS
    end

    QN["queue/nats.Driver"] --> JS
    CN["cache/nats.Driver"] --> JS

    subgraph server["NATS server"]
        STR["Stream<br/>(AllowMsgSchedules)"]
        CONS["Durable pull consumer<br/>(explicit acks)"]
        KV["KV bucket<br/>(server-side TTL)"]
    end

    JS --> STR
    STR --> CONS
    JS --> KV
```

See [NATS integration](nats.md).

## Operational concerns

- **Graceful shutdown** follows the same shape everywhere: close the intake, drain queued work, stop
  workers, release infrastructure. The [engine](engine.md#graceful-shutdown) and
  [queue store](queues.md#graceful-shutdown) both support it.
- **Observability** is structured logging (zap-based `core/logger`) plus Sentry for error reporting,
  Zabbix for metrics, and `Stats`/`Info` methods on queues for workload introspection.
- **Multi-tenancy** is the reason queues are keyed by numeric account IDs: each account gets
  independent backlog, workers, and statistics, and `Store.Reconcile` keeps the executor set in sync
  with the accounts known to the transport.
