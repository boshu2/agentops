# Content-Hash Caching Pattern

> Cache expensive operations by content hash (SHA-256), not file path. Survives renames, auto-invalidates on change.

## When to Use

- File processing pipelines (PDF extraction, image analysis, text parsing)
- Expensive LLM calls on file content (summarization, code review)
- Any operation where: same content → same result, regardless of path

## Core Pattern

### 1. Hash Computation (Chunked for Large Files)

```python
import hashlib
from pathlib import Path

_HASH_CHUNK_SIZE = 65536  # 64KB chunks

def compute_file_hash(path: Path) -> str:
    sha256 = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(_HASH_CHUNK_SIZE)
            if not chunk:
                break
            sha256.update(chunk)
    return sha256.hexdigest()
```

```go
// Go equivalent
func computeFileHash(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", fmt.Errorf("opening %s: %w", path, err)
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", fmt.Errorf("hashing %s: %w", path, err)
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### 2. File-Based Storage ({hash}.json — O(1) lookup)

```python
import json

def read_cache(cache_dir: Path, file_hash: str):
    cache_file = cache_dir / f"{file_hash}.json"
    if not cache_file.is_file():
        return None
    try:
        return json.loads(cache_file.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, ValueError, KeyError):
        return None  # Corruption → cache miss (graceful degradation)

def write_cache(cache_dir: Path, file_hash: str, data: dict):
    cache_dir.mkdir(parents=True, exist_ok=True)
    cache_file = cache_dir / f"{file_hash}.json"
    cache_file.write_text(json.dumps(data, indent=2), encoding="utf-8")
```

### 3. Service Layer (SRP: Pure Function + Cache Wrapper)

```python
# Pure processing function — no cache knowledge
def extract_text(file_path: Path) -> dict:
    """Extract text from file. Pure function."""
    # ... expensive processing ...
    return {"content": "...", "metadata": {...}}

# Cache wrapper — adds caching around pure function
def extract_with_cache(
    file_path: Path,
    *,
    cache_enabled: bool = True,
    cache_dir: Path = Path(".cache/content"),
) -> dict:
    if not cache_enabled:
        return extract_text(file_path)

    file_hash = compute_file_hash(file_path)
    cached = read_cache(cache_dir, file_hash)
    if cached is not None:
        return cached

    result = extract_text(file_path)
    write_cache(cache_dir, file_hash, result)
    return result
```

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| SHA-256 hash, not path | Survives renames, auto-invalidates on content change |
| `{hash}.json` file naming | O(1) lookup, no index file needed, easy to inspect |
| Corruption → cache miss | Graceful degradation, re-processes on next run |
| Service layer separation | Keeps processing function pure and testable |
| Lazy directory creation | `mkdir -p` on first write, no setup needed |
| Chunked hashing | Handles large files without loading into memory |

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| Path-based cache key | Breaks on file move/rename | Use content hash |
| Cache logic inside processor | SRP violation, can't test independently | Wrap as separate layer |
| `dataclasses.asdict()` with nested frozen | Breaks serialization | Manual serialization |
| No corruption handling | Corrupted cache blocks processing | Return None, re-process |
| Shared index file | Concurrency issues, bottleneck | Per-hash files |

## Integration Points

- **Research skill:** Cache firecrawl/exa results by URL content hash
- **Reverse-engineer-rpi:** Cache upstream repo analysis by commit SHA
- **Doc skill:** Cache documentation generation by source file hash
- **Any file-processing pipeline:** Add `--cache/--no-cache` flag

## Cache Cleanup

```bash
# Remove entries older than 30 days
find .cache/content/ -name "*.json" -mtime +30 -delete

# Remove all cache
rm -rf .cache/content/
```

Add to `.gitignore`:
```
.cache/content/
```
