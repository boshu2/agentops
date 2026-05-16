package doctor

import (
	"errors"
	"fmt"
	"os"
)

// Op is one of the seven canonical mutation operations. Exactly one of the
// concrete variants below is used; Mutate dispatches on the concrete type.
//
// There is deliberately no DeletePath variant: deletion is forbidden by the
// AgentOps safety envelope. A fixer that wants to "delete" a file instead uses
// Rename to move the offending file into the run's quarantine/ directory,
// leaving the final remove decision to the user.
type Op interface {
	// kind returns the canonical op name recorded in actions.jsonl.
	kind() string
}

// WriteFile creates-or-overwrites a file with the given content and mode.
type WriteFile struct {
	Content []byte
	Mode    os.FileMode
}

func (WriteFile) kind() string { return "WriteFile" }

// AppendFile appends content to a file (created if absent).
type AppendFile struct {
	Content []byte
}

func (AppendFile) kind() string { return "AppendFile" }

// Rename moves the mutated path to To via a single-filesystem atomic rename.
// This is the sanctioned stand-in for deletion (move into quarantine/).
type Rename struct {
	To string
}

func (Rename) kind() string { return "Rename" }

// Chmod performs a metadata-only permission change.
type Chmod struct {
	Mode os.FileMode
}

func (Chmod) kind() string { return "Chmod" }

// SymlinkAtomic atomically (re)points a symlink at Target. Used for .doctor/latest.
type SymlinkAtomic struct {
	Target string
}

func (SymlinkAtomic) kind() string { return "SymlinkAtomic" }

// DbExec is declared for contract completeness with the universal 7-op enum.
// `ao doctor` has no embedded SQL database, so this op is never executed; it
// returns ErrDBOpsUnused if a fixer attempts it.
type DbExec struct {
	SQL  string
	Args []any
}

func (DbExec) kind() string { return "DbExec" }

// DbMigrate is declared for contract completeness with the universal 7-op enum.
// `ao doctor` has no embedded SQL database, so this op is never executed; it
// returns ErrDBOpsUnused if a fixer attempts it.
type DbMigrate struct {
	From uint32
	To   uint32
}

func (DbMigrate) kind() string { return "DbMigrate" }

// ErrDBOpsUnused is returned when a fixer attempts a DbExec or DbMigrate op.
// AgentOps doctor has no SQLite state; these ops exist only for contract
// completeness with the universal seven-op enum.
var ErrDBOpsUnused = errors.New("DB ops not used by ao doctor")

// DescribeOp returns a short human-readable description of an op, suitable for
// dry-run output and report narratives.
func DescribeOp(op Op) string {
	switch v := op.(type) {
	case WriteFile:
		return fmt.Sprintf("WriteFile (%d bytes, mode %o)", len(v.Content), v.Mode)
	case AppendFile:
		return fmt.Sprintf("AppendFile (%d bytes)", len(v.Content))
	case Rename:
		return fmt.Sprintf("Rename -> %s", v.To)
	case Chmod:
		return fmt.Sprintf("Chmod %o", v.Mode)
	case SymlinkAtomic:
		return fmt.Sprintf("SymlinkAtomic -> %s", v.Target)
	case DbExec:
		return "DbExec (unsupported)"
	case DbMigrate:
		return fmt.Sprintf("DbMigrate %d->%d (unsupported)", v.From, v.To)
	default:
		return "unknown op"
	}
}
