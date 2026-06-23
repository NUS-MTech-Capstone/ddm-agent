package model

type ExecuteRequest struct {
	SessionID  string
	UserID     string
	HostID     string
	CRID       string
	RunID      string
	IncidentID string
	Operation  string
	Token      string
	Command    string
}

type ExecuteResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

type SessionTokenValidationRequest struct {
	SessionID      string
	UserID         string
	HostID         string
	CRID           string
	ConnectionType string
	Action         string
	Mode           string
	Token          string
}

type SessionClaims struct {
	SessionID      string
	UserID         string
	HostID         string
	CRID           string
	ConnectionType string
	Action         string
	Mode           string
}

type AutomationClaims struct {
	RunID             string
	IncidentID        string
	HostID            string
	Phase             string
	AllowedOperations []string
	PlanHash          string
}
