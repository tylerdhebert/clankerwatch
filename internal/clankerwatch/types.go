package clankerwatch

import "time"

type Profile struct {
	Name      string            `json:"name"`
	Adapter   string            `json:"adapter"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	TimeoutMS int               `json:"timeoutMs"`
	MaxRows   int               `json:"maxRows"`
	Unlocked  bool              `json:"unlocked"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type ProfileInput struct {
	Name      string            `json:"name"`
	Adapter   string            `json:"adapter"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	TimeoutMS int               `json:"timeoutMs"`
	MaxRows   int               `json:"maxRows"`
}

type UnlockInput struct {
	SecretEnv map[string]string `json:"secretEnv"`
}

type Run struct {
	ID         int64        `json:"id"`
	SessionID  string       `json:"sessionId"`
	Profile    string       `json:"profile"`
	Query      string       `json:"query"`
	Reason     string       `json:"reason"`
	Status     string       `json:"status"`
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt *time.Time   `json:"finishedAt,omitempty"`
	ExitCode   *int         `json:"exitCode,omitempty"`
	Stdout     string       `json:"stdout"`
	Stderr     string       `json:"stderr"`
	Columns    []string     `json:"columns"`
	RowCount   int          `json:"rowCount"`
	ParseError string       `json:"parseError,omitempty"`
	Notes      []Annotation `json:"notes"`
	Highlights []Annotation `json:"highlights"`
}

type RunSummary struct {
	ID         int64      `json:"id"`
	SessionID  string     `json:"sessionId"`
	Profile    string     `json:"profile"`
	Reason     string     `json:"reason"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	ExitCode   *int       `json:"exitCode,omitempty"`
	RowCount   int        `json:"rowCount"`
}

type ResultRow struct {
	RunID     int64    `json:"runId"`
	Number    int      `json:"number"`
	Cells     []string `json:"cells"`
	Highlight bool     `json:"highlight"`
}

type Annotation struct {
	ID        int64     `json:"id"`
	RunID     int64     `json:"runId"`
	RowNumber *int      `json:"rowNumber,omitempty"`
	RowEnd    *int      `json:"rowEnd,omitempty"`
	Kind      string    `json:"kind"`
	Note      string    `json:"note"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

type QueryRequest struct {
	SessionID string `json:"sessionId"`
	Profile   string `json:"profile"`
	Reason    string `json:"reason"`
	SQL       string `json:"sql"`
}

type QueryResponse struct {
	RunID     int64    `json:"runId"`
	SessionID string   `json:"sessionId"`
	Status    string   `json:"status"`
	ExitCode  *int     `json:"exitCode,omitempty"`
	Stdout    string   `json:"stdout"`
	Stderr    string   `json:"stderr"`
	Columns   []string `json:"columns"`
	RowCount  int      `json:"rowCount"`
}

type AgentSession struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SessionInput struct {
	Name string `json:"name"`
}

type AnnotationInput struct {
	Kind      string `json:"kind"`
	Note      string `json:"note"`
	RowNumber *int   `json:"rowNumber,omitempty"`
	RowEnd    *int   `json:"rowEnd,omitempty"`
	Source    string `json:"source"`
}

type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}
