package agentworker

type CgroupLimits struct {
	Root           string
	Name           string
	MemoryMaxBytes int64
}

type CgroupStatus struct {
	Applied bool   `json:"applied"`
	Path    string `json:"path,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
