package server

import "github.com/username/loganalyze/internal/model"

type uploadResponse struct {
	SessionID string `json:"session_id"`
}

type analyzeRequest struct {
	Command string `json:"command"`
	Level   string `json:"level,omitempty"`
	Regex   string `json:"regex,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
}

type analyzeResponse struct {
	Status string `json:"status"`
}

type statusResponse struct {
	Status   string `json:"status"`
	Progress string `json:"progress,omitempty"`
	Error    string `json:"error,omitempty"`
}

type sessionSummary struct {
	ID        string `json:"id"`
	FileName  string `json:"file_name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type sessionListResponse struct {
	Sessions []sessionSummary `json:"sessions"`
}

type resultsResponse struct {
	Status string        `json:"status"`
	Report *model.Report `json:"report,omitempty"`
	Events []model.Event `json:"events,omitempty"`
}
