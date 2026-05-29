package clankerwatch

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.init(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS profiles (
			name TEXT PRIMARY KEY,
			adapter TEXT NOT NULL,
			command TEXT NOT NULL,
			args_json TEXT NOT NULL,
			env_json TEXT NOT NULL,
			timeout_ms INTEGER NOT NULL,
			max_rows INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL DEFAULT '',
			profile_name TEXT NOT NULL,
			query TEXT NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			exit_code INTEGER,
			stdout TEXT NOT NULL DEFAULT '',
			stderr TEXT NOT NULL DEFAULT '',
			columns_json TEXT NOT NULL DEFAULT '[]',
			parse_error TEXT NOT NULL DEFAULT '',
			row_count INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS result_rows (
			run_id INTEGER NOT NULL,
			row_number INTEGER NOT NULL,
			cells_json TEXT NOT NULL,
			PRIMARY KEY (run_id, row_number)
		);`,
		`CREATE TABLE IF NOT EXISTS annotations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			row_number INTEGER,
			row_end INTEGER,
			kind TEXT NOT NULL,
			note TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "annotations", "row_end", "INTEGER"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "runs", "session_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.repairParsedRuns(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
	return err
}

func (s *Store) repairParsedRuns(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT runs.id, profiles.adapter, profiles.max_rows, runs.stdout
		FROM runs
		JOIN profiles ON profiles.name = runs.profile_name
		WHERE runs.parse_error != '' AND runs.stdout != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type candidate struct {
		id      int64
		adapter string
		maxRows int
		stdout  string
	}
	candidates := []candidate{}
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.adapter, &item.maxRows, &item.stdout); err != nil {
			return err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range candidates {
		parsed := ParseTable(item.adapter, item.stdout, item.maxRows)
		if parsed.Error != "" || len(parsed.Columns) == 0 {
			continue
		}
		if err := s.replaceParsedRows(ctx, item.id, parsed); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SaveProfile(ctx context.Context, input ProfileInput) (Profile, error) {
	input = normalizeProfileInput(input)
	if input.Name == "" {
		return Profile{}, errors.New("profile name is required")
	}
	if input.Adapter == "" {
		return Profile{}, errors.New("adapter is required")
	}
	if input.Command == "" {
		input.Command = defaultCommand(input.Adapter)
	}
	if input.Command == "" && input.Adapter != "generic" {
		return Profile{}, errors.New("command is required")
	}

	args, err := json.Marshal(input.Args)
	if err != nil {
		return Profile{}, err
	}
	env, err := json.Marshal(input.Env)
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO profiles
		(name, adapter, command, args_json, env_json, timeout_ms, max_rows, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			adapter = excluded.adapter,
			command = excluded.command,
			args_json = excluded.args_json,
			env_json = excluded.env_json,
			timeout_ms = excluded.timeout_ms,
			max_rows = excluded.max_rows,
			updated_at = excluded.updated_at`,
		input.Name, input.Adapter, input.Command, string(args), string(env), input.TimeoutMS, input.MaxRows, now, now)
	if err != nil {
		return Profile{}, err
	}
	profile, err := s.GetProfile(ctx, input.Name)
	if err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s *Store) ListProfiles(ctx context.Context, unlocked map[string]bool) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, adapter, command, args_json, env_json, timeout_ms, max_rows, updated_at
		FROM profiles ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := []Profile{}
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profile.Unlocked = unlocked[profile.Name]
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *Store) GetProfile(ctx context.Context, name string) (Profile, error) {
	row := s.db.QueryRowContext(ctx, `SELECT name, adapter, command, args_json, env_json, timeout_ms, max_rows, updated_at
		FROM profiles WHERE name = ?`, name)
	profile, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Profile{}, fmt.Errorf("profile %q was not found", name)
		}
		return Profile{}, err
	}
	return profile, nil
}

func scanProfile(scanner interface {
	Scan(dest ...any) error
}) (Profile, error) {
	var profile Profile
	var argsJSON, envJSON, updatedAt string
	if err := scanner.Scan(&profile.Name, &profile.Adapter, &profile.Command, &argsJSON, &envJSON, &profile.TimeoutMS, &profile.MaxRows, &updatedAt); err != nil {
		return Profile{}, err
	}
	if err := json.Unmarshal([]byte(argsJSON), &profile.Args); err != nil {
		return Profile{}, err
	}
	if err := json.Unmarshal([]byte(envJSON), &profile.Env); err != nil {
		return Profile{}, err
	}
	profile.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return profile, nil
}

func (s *Store) CreateSession(ctx context.Context, name string) (AgentSession, error) {
	slug, err := NormalizeSessionSlug(name)
	if err != nil {
		return AgentSession{}, err
	}
	id, err := newSessionID()
	if err != nil {
		return AgentSession{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, id, slug, now, now); err != nil {
		return AgentSession{}, err
	}
	return s.GetSession(ctx, id)
}

func (s *Store) EnsureSession(ctx context.Context, slug string) (AgentSession, error) {
	normalized, err := NormalizeSessionSlug(slug)
	if err != nil {
		return AgentSession{}, err
	}
	session, err := s.FindSessionBySlug(ctx, normalized)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, errSessionNotFound) {
		return AgentSession{}, err
	}
	created, err := s.CreateSession(ctx, normalized)
	if err != nil {
		if session, findErr := s.FindSessionBySlug(ctx, normalized); findErr == nil {
			return session, nil
		}
		return AgentSession{}, err
	}
	return created, nil
}

func (s *Store) FindSessionBySlug(ctx context.Context, slug string) (AgentSession, error) {
	normalized, err := NormalizeSessionSlug(slug)
	if err != nil {
		return AgentSession{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, name, created_at, updated_at FROM sessions WHERE lower(name) = ?`, normalized)
	return scanAgentSession(row)
}

func (s *Store) GetSession(ctx context.Context, id string) (AgentSession, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, created_at, updated_at FROM sessions WHERE id = ?`, strings.TrimSpace(id))
	return scanAgentSession(row)
}

func (s *Store) FindSession(ctx context.Context, value string) (AgentSession, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "latest" {
		row := s.db.QueryRowContext(ctx, `SELECT id, name, created_at, updated_at FROM sessions ORDER BY updated_at DESC, created_at DESC LIMIT 1`)
		return scanAgentSession(row)
	}
	if session, err := s.FindSessionBySlug(ctx, value); err == nil {
		return session, nil
	} else if !errors.Is(err, errSessionNotFound) {
		return AgentSession{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, name, created_at, updated_at FROM sessions WHERE id = ?`, value)
	return scanAgentSession(row)
}

func (s *Store) ListSessions(ctx context.Context) ([]AgentSession, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at, updated_at FROM sessions ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []AgentSession{}
	for rows.Next() {
		session, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *Store) TouchSession(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, now, id)
	return err
}

var errSessionNotFound = errors.New("session was not found")

func scanAgentSession(scanner interface {
	Scan(dest ...any) error
}) (AgentSession, error) {
	var session AgentSession
	var createdAt, updatedAt string
	if err := scanner.Scan(&session.ID, &session.Name, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AgentSession{}, errSessionNotFound
		}
		return AgentSession{}, err
	}
	session.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	session.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return session, nil
}

func newSessionID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("ses_%x", buf[:]), nil
}

func (s *Store) CreateRun(ctx context.Context, sessionID, profileName, query, reason, status string) (Run, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `INSERT INTO runs
		(session_id, profile_name, query, reason, status, started_at)
		VALUES (?, ?, ?, ?, ?, ?)`, sessionID, profileName, query, reason, status, now)
	if err != nil {
		return Run{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, id)
}

func (s *Store) FinishRun(ctx context.Context, id int64, status string, exitCode int, stdout string, stderr string, parsed ParsedTable) (Run, error) {
	columnsJSON, err := json.Marshal(parsed.Columns)
	if err != nil {
		return Run{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET
		status = ?, finished_at = ?, exit_code = ?, stdout = ?, stderr = ?, columns_json = ?, parse_error = ?, row_count = ?
		WHERE id = ?`, status, now, exitCode, stdout, stderr, string(columnsJSON), parsed.Error, len(parsed.Rows), id); err != nil {
		return Run{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM result_rows WHERE run_id = ?`, id); err != nil {
		return Run{}, err
	}
	for i, row := range parsed.Rows {
		cellsJSON, err := json.Marshal(row)
		if err != nil {
			return Run{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO result_rows (run_id, row_number, cells_json) VALUES (?, ?, ?)`, id, i+1, string(cellsJSON)); err != nil {
			return Run{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, id)
}

func (s *Store) replaceParsedRows(ctx context.Context, id int64, parsed ParsedTable) error {
	columnsJSON, err := json.Marshal(parsed.Columns)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET columns_json = ?, parse_error = '', row_count = ? WHERE id = ?`, string(columnsJSON), len(parsed.Rows), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM result_rows WHERE run_id = ?`, id); err != nil {
		return err
	}
	for i, row := range parsed.Rows {
		cellsJSON, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO result_rows (run_id, row_number, cells_json) VALUES (?, ?, ?)`, id, i+1, string(cellsJSON)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetRun(ctx context.Context, id int64) (Run, error) {
	run := Run{
		Columns:    []string{},
		Notes:      []Annotation{},
		Highlights: []Annotation{},
	}
	var startedAt string
	var finishedAt sql.NullString
	var exitCode sql.NullInt64
	var columnsJSON string
	row := s.db.QueryRowContext(ctx, `SELECT id, profile_name, query, reason, status, started_at, finished_at,
		exit_code, stdout, stderr, columns_json, parse_error, row_count, session_id FROM runs WHERE id = ?`, id)
	if err := row.Scan(&run.ID, &run.Profile, &run.Query, &run.Reason, &run.Status, &startedAt, &finishedAt,
		&exitCode, &run.Stdout, &run.Stderr, &columnsJSON, &run.ParseError, &run.RowCount, &run.SessionID); err != nil {
		return Run{}, err
	}
	run.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	if finishedAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, finishedAt.String)
		run.FinishedAt = &parsed
	}
	if exitCode.Valid {
		code := int(exitCode.Int64)
		run.ExitCode = &code
	}
	json.Unmarshal([]byte(columnsJSON), &run.Columns)
	if run.Columns == nil {
		run.Columns = []string{}
	}
	annotations, err := s.ListAnnotations(ctx, id)
	if err != nil {
		return Run{}, err
	}
	run.Notes = annotations
	for _, annotation := range annotations {
		if annotation.RowNumber != nil {
			run.Highlights = append(run.Highlights, annotation)
		}
	}
	return run, nil
}

func (s *Store) ListRuns(ctx context.Context, limit int, sessionID string) ([]RunSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, profile_name, reason, status, started_at, finished_at, exit_code, row_count, session_id FROM runs`
	args := []any{}
	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []RunSummary{}
	for rows.Next() {
		var run RunSummary
		var startedAt string
		var finishedAt sql.NullString
		var exitCode sql.NullInt64
		if err := rows.Scan(&run.ID, &run.Profile, &run.Reason, &run.Status, &startedAt, &finishedAt, &exitCode, &run.RowCount, &run.SessionID); err != nil {
			return nil, err
		}
		run.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		if finishedAt.Valid {
			parsed, _ := time.Parse(time.RFC3339Nano, finishedAt.String)
			run.FinishedAt = &parsed
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			run.ExitCode = &code
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) GetRows(ctx context.Context, runID int64, page int, pageSize int) (Page[ResultRow], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 250 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM result_rows WHERE run_id = ?`, runID).Scan(&total); err != nil {
		return Page[ResultRow]{}, err
	}

	highlighted, err := s.highlightMap(ctx, runID)
	if err != nil {
		return Page[ResultRow]{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT row_number, cells_json FROM result_rows
		WHERE run_id = ? ORDER BY row_number ASC LIMIT ? OFFSET ?`, runID, pageSize, offset)
	if err != nil {
		return Page[ResultRow]{}, err
	}
	defer rows.Close()

	result := Page[ResultRow]{Items: []ResultRow{}, Page: page, PageSize: pageSize, Total: total}
	for rows.Next() {
		var item ResultRow
		var cellsJSON string
		if err := rows.Scan(&item.Number, &cellsJSON); err != nil {
			return Page[ResultRow]{}, err
		}
		item.RunID = runID
		item.Highlight = highlighted[item.Number]
		json.Unmarshal([]byte(cellsJSON), &item.Cells)
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

func (s *Store) AddAnnotation(ctx context.Context, input AnnotationInput, runID int64) (Annotation, error) {
	if input.Kind == "" {
		input.Kind = "note"
	}
	if input.Source == "" {
		input.Source = "agent"
	}
	if input.Note == "" {
		return Annotation{}, errors.New("note is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if input.RowNumber != nil && input.RowEnd != nil && *input.RowEnd < *input.RowNumber {
		return Annotation{}, errors.New("row end must be greater than or equal to row start")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO annotations (run_id, row_number, row_end, kind, note, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, runID, nullableInt(input.RowNumber), nullableInt(input.RowEnd), input.Kind, input.Note, input.Source, now)
	if err != nil {
		return Annotation{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Annotation{}, err
	}
	var annotation Annotation
	var createdAt string
	var rowNumber sql.NullInt64
	var rowEnd sql.NullInt64
	row := s.db.QueryRowContext(ctx, `SELECT id, run_id, row_number, row_end, kind, note, source, created_at FROM annotations WHERE id = ?`, id)
	if err := row.Scan(&annotation.ID, &annotation.RunID, &rowNumber, &rowEnd, &annotation.Kind, &annotation.Note, &annotation.Source, &createdAt); err != nil {
		return Annotation{}, err
	}
	if rowNumber.Valid {
		n := int(rowNumber.Int64)
		annotation.RowNumber = &n
	}
	if rowEnd.Valid {
		n := int(rowEnd.Int64)
		annotation.RowEnd = &n
	}
	annotation.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return annotation, nil
}

func (s *Store) ListAnnotations(ctx context.Context, runID int64) ([]Annotation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, row_number, row_end, kind, note, source, created_at
		FROM annotations WHERE run_id = ? ORDER BY created_at ASC, id ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	annotations := []Annotation{}
	for rows.Next() {
		var annotation Annotation
		var createdAt string
		var rowNumber sql.NullInt64
		var rowEnd sql.NullInt64
		if err := rows.Scan(&annotation.ID, &annotation.RunID, &rowNumber, &rowEnd, &annotation.Kind, &annotation.Note, &annotation.Source, &createdAt); err != nil {
			return nil, err
		}
		if rowNumber.Valid {
			n := int(rowNumber.Int64)
			annotation.RowNumber = &n
		}
		if rowEnd.Valid {
			n := int(rowEnd.Int64)
			annotation.RowEnd = &n
		}
		annotation.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		annotations = append(annotations, annotation)
	}
	return annotations, rows.Err()
}

func (s *Store) highlightMap(ctx context.Context, runID int64) (map[int]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT row_number, row_end FROM annotations
		WHERE run_id = ? AND row_number IS NOT NULL`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	highlighted := map[int]bool{}
	for rows.Next() {
		var rowNumber int
		var rowEnd sql.NullInt64
		if err := rows.Scan(&rowNumber, &rowEnd); err != nil {
			return nil, err
		}
		end := rowNumber
		if rowEnd.Valid {
			end = int(rowEnd.Int64)
		}
		for row := rowNumber; row <= end; row++ {
			highlighted[row] = true
		}
	}
	return highlighted, rows.Err()
}

func normalizeProfileInput(input ProfileInput) ProfileInput {
	if input.Args == nil {
		input.Args = []string{}
	}
	if input.Env == nil {
		input.Env = map[string]string{}
	}
	if input.TimeoutMS <= 0 {
		input.TimeoutMS = 30000
	}
	if input.MaxRows <= 0 {
		input.MaxRows = 1000
	}
	return input
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
