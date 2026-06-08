// practices: [design-by-contract, code-complete]

// Package skillsresolve is the MECE half of the skill-corpus resolver
// (cp-skill-resolver-mece-dry, promoted from control-plane/bin/skill-resolve).
//
// It audits the skills/ source tree for two corpus-quality properties:
//   - Mutually Exclusive (ME): no two skills overlap. Overlapping/near-duplicate
//     skills are surfaced as merge candidates (feeds the prune work, cp-dkf).
//   - Collectively Exhaustive (CE): no coverage gaps. Thin or description-less
//     SKILL.md files are flagged as coverage-quality proxies (full CE needs the
//     domain taxonomy; this is the mechanical lower bound).
//
// The deployment-DRY half (which live-tier symlink backs each name) is an
// operator-side concern and stays in control-plane/bin/skill-resolve.
package skillsresolve

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/boshu2/agentops/cli/internal/skillshealth"
)

// DefaultOverlapThreshold is the description-token Jaccard at or above which two
// skills are reported as ME overlap candidates.
const DefaultOverlapThreshold = 0.45

// DefaultThinBytes is the SKILL.md size below which a skill is a CE coverage flag.
const DefaultThinBytes = 600

// stemMinJaccard is the lower Jaccard bar applied only when two skills already
// share a name-family stem (e.g. testing-fuzzing / testing-golden-artifacts).
const stemMinJaccard = 0.25

// Options configures Resolve.
type Options struct {
	SkillsDir        string
	OverlapThreshold float64 // 0 -> DefaultOverlapThreshold
	ThinBytes        int     // 0 -> DefaultThinBytes
}

// Overlap is one ME merge-candidate pair.
type Overlap struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Jaccard    float64 `json:"jaccard"`
	SharedStem bool    `json:"shared_stem"`
}

// CoverageGap is one CE flag: a thin or description-less skill.
type CoverageGap struct {
	Name    string `json:"name"`
	Size    int    `json:"size"`
	HasDesc bool   `json:"has_desc"`
}

// Report is the resolver result.
type Report struct {
	Generated    string        `json:"generated"`
	SkillsCount  int           `json:"skills_count"`
	Overlaps     []Overlap     `json:"me_candidate_overlaps"`
	CoverageGaps []CoverageGap `json:"ce_coverage_flags"`
}

var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "use": true, "used": true,
	"using": true, "when": true, "via": true, "per": true, "into": true, "from": true,
	"its": true, "are": true, "this": true, "that": true, "your": true, "you": true,
	"not": true, "skill": true, "skills": true,
}

type skill struct {
	name string
	size int
	desc string
	toks map[string]bool
}

// Resolve walks opts.SkillsDir and returns the ME/CE report. It mutates nothing.
func Resolve(opts Options) (*Report, error) {
	thresh := opts.OverlapThreshold
	if thresh == 0 {
		thresh = DefaultOverlapThreshold
	}
	thin := opts.ThinBytes
	if thin == 0 {
		thin = DefaultThinBytes
	}

	entries, err := os.ReadDir(opts.SkillsDir)
	if err != nil {
		return nil, err
	}

	var skills []skill
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		mdPath := filepath.Join(opts.SkillsDir, e.Name(), "SKILL.md")
		content, err := os.ReadFile(mdPath)
		if err != nil {
			continue // not a skill (e.g. _fixtures) — no SKILL.md
		}
		fm := skillshealth.ParseFrontmatter(string(content))
		name := fm["name"]
		if name == "" {
			name = e.Name()
		}
		desc := fm["description"]
		skills = append(skills, skill{
			name: name,
			size: len(content),
			desc: desc,
			toks: tokenize(strings.ReplaceAll(name, "-", " ") + " " + desc),
		})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].name < skills[j].name })

	report := &Report{
		Generated:    time.Now().UTC().Format(time.RFC3339),
		SkillsCount:  len(skills),
		Overlaps:     []Overlap{},
		CoverageGaps: []CoverageGap{},
	}

	for i := range skills {
		a := skills[i]
		if a.size < thin || a.desc == "" {
			report.CoverageGaps = append(report.CoverageGaps, CoverageGap{
				Name: a.name, Size: a.size, HasDesc: a.desc != "",
			})
		}
		if len(a.toks) == 0 {
			continue
		}
		stemA := stem(a.name)
		for j := i + 1; j < len(skills); j++ {
			b := skills[j]
			if len(b.toks) == 0 {
				continue
			}
			jac := jaccard(a.toks, b.toks)
			sharedStem := len(stemA) >= 3 && stemA == stem(b.name)
			if jac >= thresh || (sharedStem && jac >= stemMinJaccard) {
				report.Overlaps = append(report.Overlaps, Overlap{
					A: a.name, B: b.name, Jaccard: round2(jac), SharedStem: sharedStem,
				})
			}
		}
	}
	sort.SliceStable(report.Overlaps, func(i, j int) bool {
		return report.Overlaps[i].Jaccard > report.Overlaps[j].Jaccard
	})
	return report, nil
}

func stem(name string) string {
	if i := strings.IndexByte(name, '-'); i >= 0 {
		return name[:i]
	}
	return name
}

func tokenize(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(t) >= 3 && !stopwords[t] {
			out[t] = true
		}
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
