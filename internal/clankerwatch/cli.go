package clankerwatch

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func Main(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}
	switch args[0] {
	case "serve":
		return serveCommand(args[1:], stdout, stderr)
	case "status":
		return statusCommand(args[1:], stdout, stderr)
	case "profile":
		return profileCommand(args[1:], stdout, stderr)
	case "query":
		return queryCommand(args[1:], stdout, stderr)
	case "annotate":
		return annotateCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "cwatch")
	fmt.Fprintln(w, "  serve [--host 127.0.0.1] [--port 48731]")
	fmt.Fprintln(w, "  status [--json]")
	fmt.Fprintln(w, "  profile list [--json]")
	fmt.Fprintln(w, "  profile show <name> [--json]")
	fmt.Fprintln(w, "  query <profile> --session <slug> --reason <text> --sql <sql> [--json]")
	fmt.Fprintln(w, "  query <profile> --session <slug> --reason <text> --file <query.sql> [--json]")
	fmt.Fprintln(w, "  query <profile> --session <slug> --reason <text> --stdin [--json]")
	fmt.Fprintln(w, "  annotate --session <slug> [<run-id>] --note <text> [--row <n> [--to <n>] | --rows <n-m>] [--json]")
}

func parseSessionFlag(raw string) (string, error) {
	return NormalizeSessionSlug(raw)
}

func lookupSession(slug string) (AgentSession, error) {
	normalized, err := NormalizeSessionSlug(slug)
	if err != nil {
		return AgentSession{}, err
	}
	var session AgentSession
	status, err := requestJSON(http.MethodGet, "/api/sessions/"+url.PathEscape(normalized), nil, &session)
	if err != nil {
		return AgentSession{}, err
	}
	if status >= 400 {
		return AgentSession{}, fmt.Errorf("session %q was not found; run a query with --session first", normalized)
	}
	return session, nil
}

func defaultAPIPort() int {
	if value := os.Getenv("PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err == nil && port > 0 {
			return port
		}
	}
	return 48731
}

func statusCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	session, err := readSession()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var health map[string]any
	if _, err := requestJSON(http.MethodGet, "/api/health", nil, &health); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(map[string]any{
			"apiBase":   session.APIBase,
			"pid":       session.PID,
			"createdAt": session.CreatedAt,
		})
		return 0
	}
	fmt.Fprintf(stdout, "api: %s\npid: %d\nstarted: %s\n", session.APIBase, session.PID, session.CreatedAt.Format(time.RFC3339))
	return 0
}

func profileCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "profile subcommand is required")
		fmt.Fprintln(stderr, "usage: cwatch profile list|show")
		return 2
	}
	switch args[0] {
	case "list":
		return profileListCommand(args[1:], stdout, stderr)
	case "show":
		return profileShowCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown profile command %q\n", args[0])
		return 2
	}
}

func profileListCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("profile list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var profiles []Profile
	if _, err := requestJSON(http.MethodGet, "/api/profiles", nil, &profiles); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(profiles)
		return 0
	}
	for _, profile := range profiles {
		state := "locked"
		if profile.Unlocked {
			state = "unlocked"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", profile.Name, profile.Adapter, state)
	}
	return 0
}

func profileShowCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "profile name is required")
		return 2
	}
	name := args[0]
	fs := flag.NewFlagSet("profile show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	var profiles []Profile
	if _, err := requestJSON(http.MethodGet, "/api/profiles", nil, &profiles); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, profile := range profiles {
		if profile.Name != name {
			continue
		}
		if *asJSON {
			_ = json.NewEncoder(stdout).Encode(profile)
			return 0
		}
		state := "locked"
		if profile.Unlocked {
			state = "unlocked"
		}
		fmt.Fprintf(stdout, "name: %s\nadapter: %s\ncommand: %s\nstate: %s\ntimeout_ms: %d\nmax_rows: %d\n",
			profile.Name, profile.Adapter, profile.Command, state, profile.TimeoutMS, profile.MaxRows)
		if len(profile.Args) > 0 {
			fmt.Fprintln(stdout, "args:")
			for _, arg := range profile.Args {
				fmt.Fprintf(stdout, "  %s\n", arg)
			}
		}
		if len(profile.Env) > 0 {
			fmt.Fprintln(stdout, "env:")
			for key := range profile.Env {
				fmt.Fprintf(stdout, "  %s=<redacted>\n", key)
			}
		}
		return 0
	}
	fmt.Fprintf(stderr, "profile %q was not found\n", name)
	return 1
}

func serveCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	host := fs.String("host", "127.0.0.1", "server host")
	port := fs.Int("port", defaultAPIPort(), "server port")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	path, err := dbPath()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	store, err := OpenStore(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer store.Close()

	addr := *host + ":" + strconv.Itoa(*port)
	api := NewAPI(store)
	server, listener, err := api.Serve(addr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	info := SessionInfo{
		APIBase:   "http://" + listener.Addr().String(),
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}
	if err := writeSession(info); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	encoded, _ := json.Marshal(info)
	fmt.Fprintf(stdout, "CWATCH_SERVER %s\n", encoded)
	fmt.Fprintf(stderr, "clankerwatch api listening at %s\n", info.APIBase)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func queryCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "profile is required")
		return 2
	}
	profile := args[0]
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	session := fs.String("session", "", "investigation session slug")
	reason := fs.String("reason", "", "why this query is being run")
	reasonFile := fs.String("reason-file", "", "path to a file containing the query reason")
	sqlText := fs.String("sql", "", "sql to run")
	file := fs.String("file", "", "path to a sql file")
	fromStdin := fs.Bool("stdin", false, "read sql from stdin")
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	query := strings.TrimSpace(*sqlText)
	if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		query = string(data)
	}
	if *fromStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		query = string(data)
	}
	queryReason := *reason
	if *reasonFile != "" {
		data, err := os.ReadFile(*reasonFile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		queryReason = string(data)
	}
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(stderr, "--sql, --file, or --stdin is required")
		return 2
	}
	if strings.TrimSpace(queryReason) == "" {
		fmt.Fprintln(stderr, "--reason is required")
		return 2
	}
	sessionSlug, err := parseSessionFlag(*session)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var response QueryResponse
	status, err := requestJSON(http.MethodPost, "/api/query", QueryRequest{
		Session: sessionSlug,
		Profile: profile,
		Reason:  queryReason,
		SQL:     query,
	}, &response)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(response)
	} else {
		if len(response.Columns) > 0 && len(response.Rows) > 0 {
			printNumberedTable(stdout, response.Columns, response.Rows)
		} else if response.ParseError != "" {
			fmt.Fprint(stdout, response.Stdout)
			fmt.Fprintf(stderr, "cwatch: could not parse table output; row annotations unavailable\n")
		} else {
			fmt.Fprint(stdout, response.Stdout)
		}
		if response.Stderr != "" {
			fmt.Fprint(stderr, response.Stderr)
			if !strings.HasSuffix(response.Stderr, "\n") {
				fmt.Fprintln(stderr)
			}
		}
		fmt.Fprintln(stdout, queryFooter(response))
	}
	if response.ExitCode != nil {
		return *response.ExitCode
	}
	if status >= 400 {
		return 1
	}
	return 0
}

func queryFooter(response QueryResponse) string {
	line := fmt.Sprintf("cwatch: run %d", response.RunID)
	if response.RowCount > 0 {
		line += fmt.Sprintf(" (%d rows)", response.RowCount)
	}
	return line
}

func printNumberedTable(w io.Writer, columns []string, rows [][]string) {
	numWidth := len(strconv.Itoa(len(rows)))
	if numWidth < 1 {
		numWidth = 1
	}

	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = len(col)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	fmt.Fprintf(w, "%*s", numWidth, "#")
	for i, col := range columns {
		fmt.Fprintf(w, " | %-*s", colWidths[i], col)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "%s", strings.Repeat("-", numWidth))
	for _, cw := range colWidths {
		fmt.Fprintf(w, "-+-%-s", strings.Repeat("-", cw))
	}
	fmt.Fprintln(w)

	for rowIdx, row := range rows {
		fmt.Fprintf(w, "%*d", numWidth, rowIdx+1)
		for i := 0; i < len(columns); i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			fmt.Fprintf(w, " | %-*s", colWidths[i], cell)
		}
		fmt.Fprintln(w)
	}
}

func annotateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("annotate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	session := fs.String("session", "", "investigation session slug")
	note := fs.String("note", "", "annotation note")
	noteFile := fs.String("note-file", "", "path to a file containing the note")
	row := fs.Int("row", 0, "result row number")
	rowEnd := fs.Int("to", 0, "ending result row number for a range")
	rowsRange := fs.String("rows", "", "result row range, like 3-7")
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var runID int64
	var explicitRunID bool
	if len(fs.Args()) > 0 {
		id, err := strconv.ParseInt(fs.Args()[0], 10, 64)
		if err != nil {
			fmt.Fprintln(stderr, "run id must be a number")
			return 2
		}
		runID = id
		explicitRunID = true
	}
	sessionSlug, err := parseSessionFlag(*session)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !explicitRunID {
		agentSession, err := lookupSession(sessionSlug)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		latestID, err := resolveLatestRun(agentSession.ID)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		runID = latestID
	}
	noteValue := *note
	if *noteFile != "" {
		data, err := os.ReadFile(*noteFile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		noteValue = string(data)
	}
	if strings.TrimSpace(noteValue) == "" {
		fmt.Fprintln(stderr, "--note or --note-file is required")
		return 2
	}
	rowNumber, rowEndValue, err := parseAnnotationRows(*row, *rowEnd, *rowsRange)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var annotation Annotation
	_, err = requestJSON(http.MethodPost, fmt.Sprintf("/api/runs/%d/annotations", runID), AnnotationInput{
		Kind:      "annotation",
		Note:      noteValue,
		RowNumber: rowNumber,
		RowEnd:    rowEndValue,
		Source:    "agent",
	}, &annotation)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(annotation)
	} else {
		fmt.Fprintf(stdout, "saved annotation %d for run %d\n", annotation.ID, runID)
	}
	return 0
}

func resolveLatestRun(sessionID string) (int64, error) {
	path := "/api/runs?limit=1"
	if sessionID != "" {
		path += "&sessionId=" + sessionID
	}
	var runs []RunSummary
	if _, err := requestJSON(http.MethodGet, path, nil, &runs); err != nil {
		return 0, fmt.Errorf("could not resolve latest run: %w", err)
	}
	if len(runs) == 0 {
		return 0, fmt.Errorf("no runs found; run a query first")
	}
	return runs[0].ID, nil
}

func parseAnnotationRows(row int, rowEnd int, rowsRange string) (*int, *int, error) {
	if rowsRange != "" {
		parts := strings.Split(rowsRange, "-")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("--rows must look like 3-7")
		}
		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, nil, fmt.Errorf("--rows start must be a number")
		}
		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, nil, fmt.Errorf("--rows end must be a number")
		}
		if start <= 0 || end <= 0 || end < start {
			return nil, nil, fmt.Errorf("--rows must use positive ascending row numbers")
		}
		return &start, &end, nil
	}
	if row <= 0 {
		return nil, nil, nil
	}
	if rowEnd > 0 {
		if rowEnd < row {
			return nil, nil, fmt.Errorf("--to must be greater than or equal to --row")
		}
		return &row, &rowEnd, nil
	}
	return &row, nil, nil
}

func requestJSON(method string, path string, input any, output any) (int, error) {
	session, err := readSession()
	if err != nil {
		return 0, err
	}
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, strings.TrimRight(session.APIBase, "/")+path, body)
	if err != nil {
		return 0, err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if output != nil && len(data) > 0 {
		if err := json.Unmarshal(data, output); err != nil {
			return resp.StatusCode, err
		}
	}
	if resp.StatusCode >= 400 {
		var payload map[string]string
		if err := json.Unmarshal(data, &payload); err == nil && payload["error"] != "" {
			return resp.StatusCode, fmt.Errorf("%s", payload["error"])
		}
		return resp.StatusCode, nil
	}
	return resp.StatusCode, nil
}
