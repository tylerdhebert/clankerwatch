package clankerwatch

import (
	"encoding/csv"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("CWATCH_TEST_HELPER") != "csv" {
		return
	}
	_, _ = io.ReadAll(os.Stdin)
	writer := csv.NewWriter(os.Stdout)
	_ = writer.Write([]string{"id", "name"})
	_ = writer.Write([]string{"1", "generic"})
	writer.Flush()
	os.Exit(0)
}

func TestAPIDoesNotRequireTokenAndRestrictsCORS(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "clankerwatch.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	api := NewAPI(store)
	server := httptest.NewServer(api.Handler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/profiles", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("allow origin = %q, want vite origin", got)
	}

	req, err = http.NewRequest(http.MethodGet, server.URL+"/api/profiles", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://example.com")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin for disallowed origin: %q", got)
	}
}

func TestAPIRunQueryWithGenericCommand(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "clankerwatch.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveProfile(t.Context(), ProfileInput{
		Name:    "generic",
		Adapter: "generic",
		Command: exe,
		Args:    []string{"-test.run=TestHelperProcess"},
		Env:     map[string]string{"CWATCH_TEST_HELPER": "csv"},
	}); err != nil {
		t.Fatal(err)
	}

	api := NewAPI(store)
	response, status, err := api.RunQuery(t.Context(), QueryRequest{Session: "smoke-test", Profile: "generic", Reason: "smoke", SQL: "select 1"})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if response.ExitCode == nil || *response.ExitCode != 0 {
		t.Fatalf("response = %#v", response)
	}
	if response.RowCount != 1 || len(response.Columns) != 2 || !strings.Contains(response.Stdout, "generic") {
		t.Fatalf("response = %#v", response)
	}
}
