# Language-Specific Archaeology

## Rust

```bash
# Entry points
rg "fn main\(\)" src/
rg "#\[tokio::main\]|#\[async_std::main\]" src/

# CLI structure (clap)
rg "#\[derive\(.*Parser.*\)\]" src/ -A 20
rg "Subcommand|Args" src/

# Key types
rg "^pub struct |^pub enum " src/
rg "impl .* for " src/  # Trait implementations

# Error handling
rg "thiserror|anyhow|eyre" Cargo.toml
rg "#\[error\(" src/

# Config
rg "serde::Deserialize" src/ -A 10
rg "env::var|std::env" src/

# Database
rg "rusqlite|sqlx|diesel" Cargo.toml
rg "query|execute|prepare" src/
```

## TypeScript/JavaScript

```bash
# Entry points
rg "export default|module\.exports" src/index.ts src/main.ts
rg "app\.(get|post|use)\(" src/

# CLI structure
rg "program\.(command|option|argument)" src/
rg "yargs|commander|meow" package.json

# Key types
rg "^export (interface|type|class) " src/
rg "^interface |^type " src/

# Config
rg "process\.env\." src/
rg "config\[|getConfig|loadConfig" src/

# Database
rg "prisma|typeorm|sequelize|knex" package.json
rg "\.query\(|\.execute\(" src/
```

## Python

```bash
# Entry points
rg "if __name__ == ['\"]__main__['\"]" .
rg "@app\.(route|get|post)" src/

# CLI structure
rg "argparse|click|typer" requirements.txt setup.py pyproject.toml
rg "@click\.command|@app\.command" src/
rg "add_argument\(" src/

# Key types
rg "^class \w+.*:" src/ -A 5
rg "@dataclass|@attr\.s" src/

# Config
rg "os\.environ|os\.getenv" src/
rg "pydantic.*Settings|BaseSettings" src/

# Database
rg "sqlalchemy|psycopg|pymongo|sqlite3" requirements.txt
rg "session\.|cursor\." src/
```

## Go

```bash
# Entry points
rg "func main\(\)" cmd/ main.go
rg "http\.HandleFunc|mux\.Handle" .

# CLI structure
rg "cobra|flag\." .
rg "rootCmd|AddCommand" cmd/

# Key types
rg "^type \w+ struct" .
rg "^type \w+ interface" .

# Config
rg "os\.Getenv|viper\." .
rg "\.yaml|\.json|\.toml" . --type go

# Database
rg "database/sql|gorm|sqlx" go.mod
rg "db\.Query|db\.Exec" .
```
