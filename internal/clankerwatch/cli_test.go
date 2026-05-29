package clankerwatch

import "testing"

func TestParseAnnotationRows(t *testing.T) {
	start, end, err := parseAnnotationRows(3, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if *start != 3 || *end != 5 {
		t.Fatalf("range = %d-%d, want 3-5", *start, *end)
	}

	start, end, err = parseAnnotationRows(0, 0, "8-10")
	if err != nil {
		t.Fatal(err)
	}
	if *start != 8 || *end != 10 {
		t.Fatalf("range = %d-%d, want 8-10", *start, *end)
	}

	start, end, err = parseAnnotationRows(4, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if *start != 4 || end != nil {
		t.Fatalf("range = %v-%v, want row 4", start, end)
	}
}

func TestParseAnnotationRowsRejectsDescendingRanges(t *testing.T) {
	if _, _, err := parseAnnotationRows(7, 3, ""); err == nil {
		t.Fatal("expected --to validation error")
	}
	if _, _, err := parseAnnotationRows(0, 0, "7-3"); err == nil {
		t.Fatal("expected --rows validation error")
	}
}

func TestQueryFooterIncludesRunID(t *testing.T) {
	got := queryFooter(QueryResponse{RunID: 42, Status: "succeeded", RowCount: 7})
	want := "cwatch: run 42 (7 rows)"
	if got != want {
		t.Fatalf("footer = %q, want %q", got, want)
	}
}
