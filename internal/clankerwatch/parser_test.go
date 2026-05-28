package clankerwatch

import "testing"

func TestParseCSV(t *testing.T) {
	parsed := ParseTable("postgres", "id,name\n1,Tyler\n2,Ada\n", 100)
	if parsed.Error != "" {
		t.Fatalf("parse error = %q", parsed.Error)
	}
	if len(parsed.Columns) != 2 || parsed.Columns[0] != "id" || parsed.Columns[1] != "name" {
		t.Fatalf("columns = %#v", parsed.Columns)
	}
	if len(parsed.Rows) != 2 || parsed.Rows[1][1] != "Ada" {
		t.Fatalf("rows = %#v", parsed.Rows)
	}
}

func TestParseCSVTrimsSQLiteCarriageReturns(t *testing.T) {
	parsed := ParseTable("sqlite", "effort_count\r\r\n3\r\r\n", 100)
	if parsed.Error != "" {
		t.Fatalf("parse error = %q", parsed.Error)
	}
	if len(parsed.Columns) != 1 || parsed.Columns[0] != "effort_count" {
		t.Fatalf("columns = %#v", parsed.Columns)
	}
	if len(parsed.Rows) != 1 || parsed.Rows[0][0] != "3" {
		t.Fatalf("rows = %#v", parsed.Rows)
	}
}

func TestParseSQLiteCSVWithQuotedQuestionAndDoubleCarriageReturns(t *testing.T) {
	output := "id,status,prompt\r\r\n4,answered,\"Should the plan history show timestamps inline or stay terse?\"\r\r\n"
	parsed := ParseTable("sqlite", output, 100)
	if parsed.Error != "" {
		t.Fatalf("parse error = %q", parsed.Error)
	}
	if len(parsed.Rows) != 1 || parsed.Rows[0][2] != "Should the plan history show timestamps inline or stay terse?" {
		t.Fatalf("rows = %#v", parsed.Rows)
	}
}

func TestParseSQLServerTSV(t *testing.T) {
	output := "id\tname\r\n--\t----\r\n1\tTyler\r\n(1 rows affected)\r\n"
	parsed := ParseTable("sqlserver", output, 100)
	if parsed.Error != "" {
		t.Fatalf("parse error = %q", parsed.Error)
	}
	if len(parsed.Rows) != 1 || parsed.Rows[0][1] != "Tyler" {
		t.Fatalf("rows = %#v", parsed.Rows)
	}
}
