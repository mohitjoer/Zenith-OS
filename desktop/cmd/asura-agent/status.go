package main

import "time"

// Status is the full daemon report for `status --json`, the Hub page, and
// /api/status.
type Status struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	Port    int  `json:"port"`
	// Ready is true when the chat can actually answer, which is when the engine is
	// configured and its program exists. It drives the first-run prompt in the
	// sidebar. Engine.Reason says in plain words what is missing when it is false.
	Ready  bool         `json:"ready"`
	Vault  VaultStatus  `json:"vault"`
	Engine EngineStatus `json:"engine"`
	Agents []Agent      `json:"agents"`
}

// VaultStatus is the vault summary embedded in Status.
type VaultStatus struct {
	Path        string    `json:"path"`
	Exists      bool      `json:"exists"`
	Files       int       `json:"files"`
	LastIndexed time.Time `json:"lastIndexed"`
}

// BuildStatus assembles the report. Running is probed over the unix socket
// (pingDaemon, in server.go) so the answer reflects a live daemon, not just the
// enabled gate.
func BuildStatus(cfg Config) Status {
	files, last, exists := VaultStats()
	eng := engineStatus(cfg)
	return Status{
		Enabled: cfg.Enabled,
		Running: pingDaemon(),
		Port:    cfg.Port,
		Ready:   eng.Available,
		Vault: VaultStatus{
			Path:        VaultDir(),
			Exists:      exists,
			Files:       files,
			LastIndexed: last,
		},
		Engine: eng,
		Agents: DetectAgents(),
	}
}
