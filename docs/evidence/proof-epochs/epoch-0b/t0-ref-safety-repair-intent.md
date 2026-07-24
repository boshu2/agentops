# T0 reference-safety repair invocation exact intent

Consume the immutable interrupted transition-schema result
`4255446100319be16e31553e51278d94f029d6b4942a4f90a27ae9293c47978e`
and perform one new, bounded experiment that repairs only the unresolved
repository-reference alias concern. The four prior FAILs, the NOT_PROVEN
result, and all rejected subjects must remain byte-immutable.

## Required criteria

- **T0RS-1 — Lexical reference identity:** every consumed repository reference
  is a normalized, nonempty POSIX-relative path with no absolute form,
  backslash, empty segment, dot segment, or parent segment.
- **T0RS-2 — No symlink aliases:** the repository root walks every existing
  reference component with no symlinked parent or final component before
  resolving containment. Direct, parent, internal-target, and external-target
  aliases are rejected; ordinary contained paths still pass the full simulated
  epoch-1 transition.
- **T0RS-3 — Immutable terminal history:** all four prior FAILs, the interrupted
  NOT_PROVEN result, and their subject/report artifacts remain exact-byte
  unchanged.

## Declared exclusions

- Repairing any T1 exact-kernel behavior.
- Performing a real epoch-1 activation.
- Changing any prior verdict, subject manifest, or report.
- Changing bootstrap recorder behavior or proof-contract authority.
- Go CLI G0 through G2 implementation.
- Installed skill links, external systems, credentials, or release channels.
