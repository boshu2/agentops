# Common Architecture Patterns

This file helps you recognize common system shapes quickly. Use these signals to decide where to look next and how to trace data flow.

## Layered (MVC / N-tier)
Signals:
- Folders like controllers/, services/, repositories/ or handlers/, domain/, infra/.
- Flow: route -> controller -> service -> repository.

Quick checks:
```bash
rg "controller|service|repository|handler" src/
rg "route|router" src/
```

## Hexagonal (Ports / Adapters)
Signals:
- domain/ or core/ with explicit interfaces (ports).
- adapters/ for external systems (db, http, queue).

Quick checks:
```bash
rg "port|adapter|gateway" src/
rg "trait .*Port|interface .*Port" src/
```

## Event-Driven / CQRS
Signals:
- events/, handlers/, consumers/ directories.
- Terms like event, handler, command, query, projector.

Quick checks:
```bash
rg "event|handler|consumer|queue|message" src/
rg "command|query|projection|projector" src/
```

## Monorepo / Multi-Service
Signals:
- apps/, packages/, services/, crates/ at the root.
- Multiple package.json or Cargo.toml files.

Quick checks:
```bash
rg --files -g '*/package.json'
rg --files -g '*/Cargo.toml'
```

## Plugin / Extension System
Signals:
- plugins/, extensions/, hooks/ folders.
- Registration or discovery code paths.

Quick checks:
```bash
rg "plugin|extension|hook|register" src/
```

## Background Jobs / Schedulers
Signals:
- jobs/, workers/, cron/ directories.
- Scheduling libs (bull, sidekiq, celery, cron, agenda).

Quick checks:
```bash
rg "job|worker|cron|schedule" src/
rg "bull|celery|sidekiq|agenda" .
```

## Client-Server Split
Signals:
- api/ routes + client/ or ui/ for frontend.
- Strong separation between request handling and domain logic.

Quick checks:
```bash
rg "api/|routes|controllers" src/
rg "client|ui|web|frontend" src/
```
