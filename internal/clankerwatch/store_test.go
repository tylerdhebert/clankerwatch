package clankerwatch

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreProfileRunRowsAndAnnotations(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "clankerwatch.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	profile, err := store.SaveProfile(ctx, ProfileInput{
		Name:    "local",
		Adapter: "sqlite",
		Args:    []string{"app.db"},
		Env:     map[string]string{"A": "B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Env["A"] != "B" || profile.TimeoutMS == 0 || profile.MaxRows == 0 {
		t.Fatalf("profile defaults/env were not saved: %#v", profile)
	}

	session, err := store.CreateSession(ctx, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateRun(ctx, session.ID, "local", "select 1", "check", "running")
	if err != nil {
		t.Fatal(err)
	}
	if run.SessionID != session.ID {
		t.Fatalf("run session = %q, want %q", run.SessionID, session.ID)
	}
	run, err = store.FinishRun(ctx, run.ID, "succeeded", 0, "value\n1\n", "", ParsedTable{
		Columns: []string{"value"},
		Rows:    [][]string{{"1"}, {"2"}, {"3"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.RowCount != 3 || run.Columns[0] != "value" {
		t.Fatalf("run parse state = %#v", run)
	}

	n := 1
	end := 2
	if _, err := store.AddAnnotation(ctx, AnnotationInput{Kind: "annotation", RowNumber: &n, RowEnd: &end, Note: "important"}, run.ID); err != nil {
		t.Fatal(err)
	}
	page, err := store.GetRows(ctx, run.ID, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 || !page.Items[0].Highlight || !page.Items[1].Highlight || page.Items[2].Highlight {
		t.Fatalf("rows page = %#v", page)
	}
}

func TestStoreEnsureSessionIsCaseInsensitive(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "clankerwatch.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	created, err := store.EnsureSession(ctx, "StoreFront-Audit")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "storefront-audit" {
		t.Fatalf("name = %q, want storefront-audit", created.Name)
	}
	found, err := store.FindSessionBySlug(ctx, "STOREFRONT-audit")
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID {
		t.Fatalf("found id = %q, want %q", found.ID, created.ID)
	}
	again, err := store.EnsureSession(ctx, "storefront-audit")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != created.ID {
		t.Fatalf("ensure created duplicate session: %#v vs %#v", again, created)
	}
}
