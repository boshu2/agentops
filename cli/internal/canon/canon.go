// practices: [ddd-bounded-context, knowledge-flywheel]

// Package canon implements the team-knowledge canon: the trusted set of
// learnings a team shares, where an entry is *earned* into canon rather than
// self-asserted.
//
// The moat thesis (ACFS deep dive, 2026-05-31): "reliability is a property of
// the system, not the model" → the defensible asset is curated knowledge
// earned by verification, not self-certification. A single engineer's
// .agents/learnings/ are provisional; an entry becomes team canon only when it
// is independently attested to by someone other than its author.
//
// Promotion is gated on TWO independent signals, and the same
// anti-self-certification rule applies to both:
//
//   - Citation     — another engineer used the knowledge (proves it is useful).
//   - Verification — another engineer independently checked that it holds
//     (proves it is true).
//
// An attestation made by the entry's own author never counts toward promotion.
// This generalizes the self-citation guard to every promotion signal: the
// factory may not grade its own homework.
package canon

import (
	"os"
	"os/exec"
	"strings"
)

// Identity is an engineer's attribution as recorded by git.
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// IsZero reports whether the identity carries no usable attribution.
func (i Identity) IsZero() bool {
	return strings.TrimSpace(i.Name) == "" && strings.TrimSpace(i.Email) == ""
}

// SameAs reports whether two identities denote the same engineer. Email is the
// strong key (matches when both sides carry a non-empty, equal email);
// otherwise it falls back to an exact, case-insensitive name match. Two zero
// identities are NOT considered the same engineer — absent attribution must
// never silently satisfy the cross-engineer guard.
func (i Identity) SameAs(other Identity) bool {
	ie, oe := strings.TrimSpace(i.Email), strings.TrimSpace(other.Email)
	if ie != "" && oe != "" {
		return strings.EqualFold(ie, oe)
	}
	in, on := strings.TrimSpace(i.Name), strings.TrimSpace(other.Name)
	if in == "" || on == "" {
		return false
	}
	return strings.EqualFold(in, on)
}

// gitIdentity resolves an identity from git config, falling back to the OS user
// for the name when git is unconfigured. It is the lowest-precedence tier of
// ResolveIdentity — the human fallback.
func gitIdentity() Identity {
	id := Identity{}
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		id.Name = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		id.Email = strings.TrimSpace(string(out))
	}
	if id.Name == "" {
		if u := os.Getenv("USER"); u != "" {
			id.Name = u
		} else if u := os.Getenv("USERNAME"); u != "" {
			id.Name = u
		}
	}
	return id
}

// AuthorOf extracts the author identity from a learning file's YAML
// frontmatter (author: / author_email: keys). A missing file or absent keys
// yields a zero Identity, which by SameAs semantics can never satisfy the
// cross-engineer guard — an unattributed entry cannot be promoted.
func AuthorOf(path string) Identity {
	content, err := os.ReadFile(path)
	if err != nil {
		return Identity{}
	}
	id := Identity{}
	inFrontmatter := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // end of frontmatter
		}
		if !inFrontmatter {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "author_email:"):
			id.Email = unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "author_email:")))
		case strings.HasPrefix(trimmed, "author:"):
			id.Name = unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "author:")))
		}
	}
	return id
}

func unquote(s string) string {
	return strings.Trim(s, "\"'")
}
