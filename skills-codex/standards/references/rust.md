# Rust Standards (Tier 1)

## Required
- `cargo fmt` (automatic)
- `cargo clippy` passes (no warnings)
- All public items documented (rustdoc)

## Error Handling
- Use `Result<T, E>` for fallible operations
- Implement custom errors with `thiserror` or `anyhow`
- Never `unwrap()` in library code (OK in tests/bins)
- Use `?` operator for error propagation

## Ownership & Borrowing
- Prefer references over cloning
- Use `&str` in function params over `String`
- Add explicit lifetime annotations when needed
- Clone sparingly and document why

## Common Issues
| Pattern | Problem | Fix |
|---------|---------|-----|
| `unwrap()` | Panic on None/Err | Use `?` or pattern match |
| Mutable statics | Data races | Use `once_cell` or `Mutex` |
| String allocation | Performance | Use `&str` in function params |
| Lifetime errors | Borrow checker reject | Add explicit lifetimes |
| Unsafe block | Memory unsafety | Add `// SAFETY:` comment |
| Excessive `.clone()` | Performance waste | Use references or `Cow<T>` |

## Unsafe Code
- Always add `// SAFETY:` comment explaining invariants
- Minimize unsafe scope
- Prefer safe abstractions

## Security
- Minimize `unsafe` blocks — each needs `// SAFETY:` justification
- Use `secrecy::Secret<T>` for sensitive values (prevents accidental logging)
- Validate all external input before deserialization (`serde` validators)
- Prefer `ring` or `rustls` over OpenSSL bindings

## Documentation
- All public items must have rustdoc comments (`///`)
- Include `# Examples` section in doc comments for complex APIs
- Use `#![deny(missing_docs)]` in library crates
- Run `cargo doc --no-deps` to verify doc builds

## Testing
- `cargo test` (built-in)
- `cargo test --doc` (doc tests)
- Use `#[cfg(test)]` modules
- `cargo bench` for benchmarks

## Adapter Recursion-Guard

When a kernel adapter spawns subprocesses that **could re-enter the kernel**
(e.g. a `ShellEvalRunner` that runs eval scripts which might themselves call
the same CLI that owns the runner), the adapter MUST set a guard env var on
every subprocess AND the kernel's entry function MUST refuse to run when it
observes that var set. Without this pattern the adapter recurses infinitely
and burns the build lock. In-band comments in eval scripts ("do not shell
out to me from here") do not scale across N authors — make the kernel
enforce what the comment asks for.

```rust
// Convention: <TOOL>_IN_PROGRESS=1 on every subprocess this adapter spawns;
// the kernel entry function refuses to run when it observes the var set.
pub const GUARD_ENV: &str = "MY_KERNEL_IN_PROGRESS";

fn build_command(repo_root: &Path, command: &str) -> Command {
    let mut cmd = Command::new("sh");
    cmd.arg("-c").arg(command)
        .current_dir(repo_root)
        .env(GUARD_ENV, "1");  // propagate to children
    cmd
}

pub fn kernel_entry(...) -> Result<..., KernelError> {
    if std::env::var(GUARD_ENV).is_ok() {
        return Err(KernelError::Adapter(format!(
            "refusing recursion: {GUARD_ENV} is set"
        )));
    }
    // ... real work
}
```

**Test both ends** with a hermetic pair:
- `kernel_entry()` returns the recursion error when the env var is set.
- The subprocess `Command` carries `GUARD_ENV=1` (assert via
  `cmd.get_envs()` — no spawn needed, no test-parallelism hazard).

**Evidence:** mt-olympus commit `97e16fe` shipped this pattern after a
two-agent strategic duel surfaced the recursion bug in a
shipped-this-session adapter. The eval scripts had warned each other
manually; the kernel adapter had no enforcement.
