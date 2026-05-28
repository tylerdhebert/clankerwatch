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
	"path/filepath"
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
	case "session":
		return sessionCommand(args[1:], stdout, stderr)
	case "profile":
		return profileCommand(args[1:], stdout, stderr)
	case "query":
		return queryCommand(args[1:], stdout, stderr)
	case "annotate":
		return annotateCommand(args[1:], stdout, stderr, "note")
	case "highlight":
		return annotateCommand(args[1:], stdout, stderr, "highlight")
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
	fmt.Fprintln(w, "  session create [--name <name>] [--json]")
	fmt.Fprintln(w, "  session reattach [<id|name|latest>] [--json]")
	fmt.Fprintln(w, "  session list [--json]")
	fmt.Fprintln(w, "  profile list [--json]")
	fmt.Fprintln(w, "  profile show <name> [--json]")
	fmt.Fprintln(w, "  query <profile> --reason <text> --sql <sql> [--json]")
	fmt.Fprintln(w, "  query <profile> --reason <text> --file <query.sql> [--json]")
	fmt.Fprintln(w, "  query <profile> --reason <text> --stdin [--json]")
	fmt.Fprintln(w, "  annotate <run-id> --note <text> [--row <n> [--to <n>] | --rows <n-m>] [--json]")
	fmt.Fprintln(w, "  highlight <run-id> --row <n> [--to <n>] --note <text> [--json]")
	fmt.Fprintln(w, "  highlight <run-id> --rows <n-m> --note <text> [--json]")
}

func sessionCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "session subcommand is required")
		fmt.Fprintln(stderr, "usage: cwatch session create|reattach|list")
		return 2
	}
	switch args[0] {
	case "create":
		return sessionCreateCommand(args[1:], stdout, stderr)
	case "reattach":
		return sessionReattachCommand(args[1:], stdout, stderr)
	case "list":
		return sessionListCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown session command %q\n", args[0])
		return 2
	}
}

func sessionCreateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("session create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", defaultSessionName(), "session name")
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var session AgentSession
	if _, err := requestJSON(http.MethodPost, "/api/sessions", SessionInput{Name: *name}, &session); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printSession(stdout, session, *asJSON)
	return 0
}

func sessionReattachCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	target := "latest"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("session reattach", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var session AgentSession
	if _, err := requestJSON(http.MethodGet, "/api/sessions/"+url.PathEscape(target), nil, &session); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printSession(stdout, session, *asJSON)
	return 0
}

func sessionListCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("session list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var sessions []AgentSession
	if _, err := requestJSON(http.MethodGet, "/api/sessions", nil, &sessions); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(sessions)
		return 0
	}
	for _, session := range sessions {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", session.ID, session.Name, session.UpdatedAt.Format(time.RFC3339))
	}
	return 0
}

func printSession(stdout io.Writer, session AgentSession, asJSON bool) {
	if asJSON {
		_ = json.NewEncoder(stdout).Encode(session)
		return
	}
	fmt.Fprintf(stdout, "$env:CWATCH_SESSION_ID=%s\n", powershellQuote(session.ID))
	fmt.Fprintf(stdout, "$env:CWATCH_SESSION_NAME=%s\n", powershellQuote(session.Name))
}

func powershellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func activeSessionID() string {
	return os.Getenv("CWATCH_SESSION_ID")
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

func defaultSessionName() string {
	dir, err := os.Getwd()
	if err != nil {
		return "agent session"
	}
	name := strings.TrimSpace(filepath.Base(dir))
	if name == "" {
		return "agent session"
	}
	return name + " / agent / " + time.Now().Format("3:04 PM")
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

	var response QueryResponse
	status, err := requestJSON(http.MethodPost, "/api/query", QueryRequest{
		SessionID: activeSessionID(),
		Profile:   profile,
		Reason:    queryReason,
		SQL:       query,
	}, &response)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(response)
	} else {
		fmt.Fprint(stdout, response.Stdout)
		fmt.Fprintln(stderr, queryMetadataLine(response))
		if response.Stderr != "" {
			fmt.Fprint(stderr, response.Stderr)
			if !strings.HasSuffix(response.Stderr, "\n") {
				fmt.Fprintln(stderr)
			}
		}
	}
	if response.ExitCode != nil {
		return *response.ExitCode
	}
	if status >= 400 {
		return 1
	}
	return 0
}

func queryMetadataLine(response QueryResponse) string {
	line := fmt.Sprintf("cwatch run %d %s", response.RunID, response.Status)
	if response.RowCount > 0 {
		line += fmt.Sprintf(" (%d rows)", response.RowCount)
	}
	return line
}

func annotateCommand(args []string, stdout io.Writer, stderr io.Writer, kind string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "run id is required")
		return 2
	}
	runID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Fprintln(stderr, "run id must be a number")
		return 2
	}
	fs := flag.NewFlagSet(kind, flag.ContinueOnError)
	fs.SetOutput(stderr)
	note := fs.String("note", "", "annotation note")
	noteFile := fs.String("note-file", "", "path to a file containing the note")
	row := fs.Int("row", 0, "result row number")
	rowEnd := fs.Int("to", 0, "ending result row number for a range")
	rowsRange := fs.String("rows", "", "result row range, like 3-7")
	asJSON := fs.Bool("json", false, "print structured response")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
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
	if kind == "highlight" && rowNumber == nil {
		fmt.Fprintln(stderr, "--row or --rows is required for highlight")
		return 2
	}
	var annotation Annotation
	_, err = requestJSON(http.MethodPost, fmt.Sprintf("/api/runs/%d/annotations", runID), AnnotationInput{
		Kind:      kind,
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
		fmt.Fprintf(stdout, "saved %s %d for run %d\n", kind, annotation.ID, runID)
	}
	return 0
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
