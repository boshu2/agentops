# Service Containers

## PostgreSQL

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: testdb
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4
      - run: cargo test
        env:
          DATABASE_URL: postgres://test:test@localhost:5432/testdb
```

---

## Redis

```yaml
services:
  redis:
    image: redis:7
    ports:
      - 6379:6379
    options: >-
      --health-cmd "redis-cli ping"
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
```

---

## MySQL

```yaml
services:
  mysql:
    image: mysql:8
    env:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: testdb
    ports:
      - 3306:3306
    options: >-
      --health-cmd "mysqladmin ping"
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
```

---

## Multiple Services

```yaml
services:
  postgres:
    image: postgres:16
    env:
      POSTGRES_PASSWORD: test
    ports:
      - 5432:5432

  redis:
    image: redis:7
    ports:
      - 6379:6379

  minio:
    image: minio/minio
    ports:
      - 9000:9000
    env:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
```

---

## Backend Matrix Testing

```yaml
strategy:
  matrix:
    backend: [postgres, sqlite, mysql]
    include:
      - backend: postgres
        db_url: postgres://test:test@localhost:5432/testdb
      - backend: sqlite
        db_url: sqlite://./test.db
      - backend: mysql
        db_url: mysql://root:root@localhost:3306/testdb

services:
  postgres:
    image: ${{ matrix.backend == 'postgres' && 'postgres:16' || '' }}
    # ... config

steps:
  - run: cargo test
    env:
      DATABASE_URL: ${{ matrix.db_url }}
```

---

## Wait for Service

```yaml
- name: Wait for Postgres
  run: |
    until pg_isready -h localhost -p 5432; do
      echo "Waiting for postgres..."
      sleep 1
    done
```
