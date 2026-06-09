# Example Exploration Sessions

These are short, repeatable walkthroughs you can follow in a new repo. The goal is a fast, accurate mental model.

## Example 1: Next.js SaaS

1) Docs first
```bash
cat AGENTS.md README.md | head -80
```

2) Entry points and routes
```bash
ls -la src/app
rg -n "route.ts" src/app/api
```

3) Config + env
```bash
rg -n "@t3-oss/env-nextjs|process\.env" src
```

4) Data flow (route -> service -> db)
```bash
rg -n "export async function GET|POST" src/app/api
rg -n "db\.|drizzle" src/lib
```

Deliverable: A 1-page architecture summary with key routes, services, and storage.

---

## Example 2: Rust CLI

1) Entry point
```bash
rg -n "fn main" --type rust src
```

2) CLI command structure
```bash
rg -n "clap::|derive\(.*Parser" src
```

3) Data flow
```bash
rg -n "reqwest|http" src
rg -n "rusqlite|tantivy" src
```

Deliverable: Map from CLI args -> command handler -> API/db layer.

---

## Example 3: Python FastAPI

1) Entry points and routes
```bash
rg -n "FastAPI\(|@app\." src
```

2) Config + settings
```bash
rg -n "BaseSettings|pydantic" src
rg -n "os\.environ|os\.getenv" src
```

3) Storage + integrations
```bash
rg -n "sqlalchemy|psycopg|redis" src
```

Deliverable: Identify route modules, business services, and storage adapters.
