package model

type ExecuteRequest struct {
	SessionID string
	UserID    string
	HostID    string
	CRID      string
	Token     string
	Command   string
}

type ExecuteResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}
