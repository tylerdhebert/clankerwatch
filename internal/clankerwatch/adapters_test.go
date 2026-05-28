package clankerwatch

import "testing"

func TestBuildCommandRedactedProfilePlusSecretEnv(t *testing.T) {
	profile := Profile{
		Adapter: "postgres",
		Args:    []string{"--set", "ON_ERROR_STOP=1"},
		Env:     map[string]string{"PGAPPNAME": "clankerwatch"},
	}
	spec := BuildCommand(profile, "query.sql", map[string]string{"DATABASE_URL": "postgres://secret"})

	if spec.Name != "psql" {
		t.Fatalf("command = %q, want psql", spec.Name)
	}
	wantArgs := []string{"--set", "ON_ERROR_STOP=1", "--csv", "--file", "query.sql"}
	if len(spec.Args) != len(wantArgs) {
		t.Fatalf("args = %#v, want %#v", spec.Args, wantArgs)
	}
	for i := range wantArgs {
		if spec.Args[i] != wantArgs[i] {
			t.Fatalf("args = %#v, want %#v", spec.Args, wantArgs)
		}
	}
	if spec.Env["PGAPPNAME"] != "clankerwatch" || spec.Env["DATABASE_URL"] != "postgres://secret" {
		t.Fatalf("env was not merged: %#v", spec.Env)
	}
}

func TestBuildCommandSQLiteOrdersFlagsBeforeDatabase(t *testing.T) {
	profile := Profile{Adapter: "sqlite", Args: []string{"app.db"}}
	spec := BuildCommand(profile, "query.sql", nil)

	want := []string{"-readonly", "-cmd", ".headers on", "-cmd", ".mode csv", "app.db", ".read 'query.sql'"}
	if len(spec.Args) != len(want) {
		t.Fatalf("args = %#v, want %#v", spec.Args, want)
	}
	for i := range want {
		if spec.Args[i] != want[i] {
			t.Fatalf("args = %#v, want %#v", spec.Args, want)
		}
	}
}
