# Domain Checklists

Use these as starting points. Tailor to the stack and risk profile.

---

## Security

### Checklist
- [ ] Input validation / sanitization
- [ ] SQL/NoSQL injection prevention
- [ ] XSS prevention
- [ ] CSRF protection
- [ ] Authentication / authorization
- [ ] Secrets management (no hardcoded keys)
- [ ] Path traversal prevention
- [ ] Error message information leakage
- [ ] Encryption at rest and in transit
- [ ] Dependency vulnerabilities

### Quick Grep
```bash
# Panics and unsafe unwraps (Rust)
rg -n "unwrap\(\)|expect\(|panic!|unreachable!" --type rust

# Code injection (JS/Python)
rg -n "eval\(|exec\(|Function\(" --type js --type py

# DOM injection (Web)
rg -n "innerHTML|outerHTML|dangerouslySetInnerHTML|v-html" --type ts --type js --type vue

# Hardcoded secrets
rg -n "password|secret|api_key|private_key|token" --type-not lock -i
rg -n "sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{36}"  # OpenAI/GitHub

# SQL building (potential injection)
rg -n "format!.*SELECT|execute.*\+" --type rust
rg -n 'f"SELECT|f\'SELECT' --type py
```

### Tools
```bash
cargo audit                    # Rust deps
npm audit --json              # Node deps
bandit -r . -f json           # Python security
semgrep --config=p/security-audit .
gitleaks detect --source .    # Secret detection
```

---

## UX / Accessibility

### Checklist
- [ ] Keyboard navigation works end-to-end
- [ ] Screen reader support (aria-*, labels)
- [ ] Color contrast meets 4.5:1
- [ ] Focus indicators visible
- [ ] Skip links or landmarks exist
- [ ] Form labels and error messaging clear
- [ ] Loading and empty states guide users
- [ ] Mobile responsiveness verified

### Quick Grep
```bash
# Missing alt text
rg -n '<img(?![^>]*alt=)' --type html --type tsx

# ARIA issues
rg -n 'aria-hidden="true"' --type tsx | grep -i "button\|link\|input"

# Focus management
rg -n "tabIndex|tabindex" --type tsx --type html

# Forms without labels
rg -n '<input(?![^>]*(id=|aria-label))' --type html
```

### Tools
```bash
npx axe-cli https://localhost:3000    # Accessibility
npx lighthouse --only-categories=accessibility
```

---

## Performance

### Checklist
- [ ] N+1 query patterns eliminated
- [ ] Indexes on frequently queried columns
- [ ] No allocations in hot paths
- [ ] Caching for expensive operations
- [ ] No sync blocking in async code
- [ ] Pagination on large datasets
- [ ] Memory leaks (unclosed resources)
- [ ] Bundle size optimized (frontend)
- [ ] Lazy loading where appropriate

### Quick Grep
```bash
# Async in loops (potential N+1)
rg -n "for.*\.await|\.await.*for" --type rust
rg -n "await.*for|for.*await" --type ts --type js

# Missing LIMIT
rg -n "SELECT.*FROM" | grep -v -i "LIMIT\|COUNT\|EXISTS"

# Blocking calls in async
rg -n "std::fs::|std::io::" --type rust  # Should be tokio::fs

# Large allocations
rg -n "Vec::with_capacity\([0-9]{5,}\)" --type rust
rg -n "new Array\([0-9]{5,}\)" --type js
```

### Tools
```bash
cargo flamegraph                     # Rust profiling
node --prof app.js                   # Node profiling
EXPLAIN ANALYZE SELECT ...           # PostgreSQL query plan
npx webpack-bundle-analyzer          # Bundle size
```

---

## API

### Checklist
- [ ] Consistent naming (snake_case vs camelCase)
- [ ] Correct HTTP methods (GET=read, POST=create, PUT=update, DELETE=remove)
- [ ] Proper status codes (200, 201, 400, 401, 403, 404, 422, 500)
- [ ] Consistent error response format
- [ ] Pagination on list endpoints
- [ ] Rate limiting in place
- [ ] Versioning strategy (URL, header, or content-type)
- [ ] OpenAPI/Swagger documentation complete

### Quick Grep
```bash
# Mixed naming conventions
rg -n '"[a-z]+_[a-z]+".*:' --type json  # snake_case
rg -n '"[a-z]+[A-Z][a-z]+".*:' --type json  # camelCase

# Status code usage
rg -n "status.*[0-9]{3}|StatusCode::" --type rust --type ts

# Missing pagination
rg -n "GET.*\[" | grep -v "page\|limit\|cursor\|offset"
```

### Tools
```bash
npx @stoplight/spectral-cli lint openapi.yaml
npx @apidevtools/swagger-cli validate openapi.yaml
```

---

## Copy

### Checklist
- [ ] Tone consistent across UI
- [ ] No grammar/spelling errors
- [ ] CTAs (buttons) say what they do
- [ ] Error messages explain AND suggest fix
- [ ] No jargon in user-facing text
- [ ] Inclusive language used
- [ ] No placeholder text ("Lorem ipsum", "TODO")
- [ ] Empty states have guidance
- [ ] Loading states communicate progress

### Quick Grep
```bash
# Placeholder/TODO text
rg -n "lorem|ipsum|TODO|FIXME|placeholder" -i --type tsx --type html

# Vague errors
rg -n '"error"|"Error"|"failed"' | grep -v -i "message\|description"

# Hardcoded strings (i18n candidates)
rg -n '>[A-Z][a-z].{10,}</' --type tsx  # JSX text content
```

### Tools
```bash
npx cspell "**/*.{ts,tsx,md}"    # Spell check
vale .                            # Prose linter
```

---

## CLI

### Checklist
- [ ] `--help` / `-h` comprehensive
- [ ] `--version` / `-V` works
- [ ] Exit codes correct (0=success, 1=error, 2=usage)
- [ ] Error messages actionable
- [ ] Progress feedback for long operations
- [ ] `--quiet` / `--verbose` modes
- [ ] Shell completion available
- [ ] stdout=data, stderr=diagnostics
- [ ] Colors respect NO_COLOR env

### Quick Tests
```bash
# Help exists and is useful
./tool --help | head -20
./tool -h 2>&1 | grep -i "usage\|options"

# Version works
./tool --version

# Exit codes
./tool valid-command && echo "Exit: 0"
./tool --bad-flag 2>/dev/null; echo "Exit: $?"
./tool nonexistent 2>/dev/null; echo "Exit: $?"

# Color handling
NO_COLOR=1 ./tool --help | cat -v  # Should have no escapes
```

### Patterns to Check
```bash
# Exit code handling
rg -n "exit\(|process\.exit\(|std::process::exit" --type rust --type ts

# Progress indicators
rg -n "indicatif|progress|spinner" --type rust
rg -n "ora|progress|cli-progress" --type ts

# Color handling
rg -n "NO_COLOR|FORCE_COLOR|isatty" --type rust --type ts
```
