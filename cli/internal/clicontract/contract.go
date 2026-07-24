// Package clicontract projects the live Cobra tree into a stable, recursive,
// side-effect-free machine contract.
package clicontract

type Flag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Origin      string `json:"origin"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type Command struct {
	ID            string            `json:"id"`
	Path          string            `json:"path"`
	Use           string            `json:"use"`
	Short         string            `json:"short"`
	Aliases       []string          `json:"aliases,omitempty"`
	Deprecated    string            `json:"deprecated,omitempty"`
	Args          string            `json:"args"`
	Output        string            `json:"output"`
	Effects       string            `json:"effects"`
	EffectDetails []Effect          `json:"effect_details,omitempty"`
	OutputFormats []OutputFormat    `json:"output_formats,omitempty"`
	DryRun        DryRunPolicy      `json:"dry_run,omitempty"`
	ReadOnly      *bool             `json:"read_only,omitempty"`
	Flags         []Flag            `json:"flags,omitempty"`
	ExitCodes     map[string]string `json:"exit_codes"`
}
