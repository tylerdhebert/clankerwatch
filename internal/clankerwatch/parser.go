package clankerwatch

import (
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strings"
)

type ParsedTable struct {
	Columns []string
	Rows    [][]string
	Error   string
}

func ParseTable(adapter string, stdout string, maxRows int) ParsedTable {
	if strings.TrimSpace(stdout) == "" {
		return ParsedTable{}
	}
	if maxRows <= 0 {
		maxRows = 1000
	}
	switch adapter {
	case "postgres", "sqlite":
		return parseDelimited(stdout, ',', maxRows)
	case "sqlserver":
		return parseSQLServer(stdout, maxRows)
	default:
		if strings.Contains(firstNonEmptyLine(stdout), "\t") {
			return parseDelimited(stdout, '\t', maxRows)
		}
		if strings.Contains(firstNonEmptyLine(stdout), ",") {
			return parseDelimited(stdout, ',', maxRows)
		}
		return ParsedTable{Error: "raw output did not look like CSV or TSV"}
	}
}

func parseDelimited(stdout string, comma rune, maxRows int) ParsedTable {
	stdout = strings.ReplaceAll(stdout, "\r\r\n", "\n")
	stdout = strings.ReplaceAll(stdout, "\r\n", "\n")
	reader := csv.NewReader(strings.NewReader(stdout))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records := make([][]string, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ParsedTable{Error: err.Error()}
		}
		if len(record) == 0 {
			continue
		}
		for i := range record {
			record[i] = strings.TrimSuffix(record[i], "\r")
		}
		records = append(records, record)
		if len(records) > maxRows+1 {
			break
		}
	}
	if len(records) == 0 {
		return ParsedTable{}
	}
	return ParsedTable{Columns: records[0], Rows: records[1:]}
}

var sqlcmdCountLine = regexp.MustCompile(`^\(\d+\s+rows?\s+affected\)$`)
var sqlcmdRuleLine = regexp.MustCompile(`^[\s\-\+\t]+$`)

func parseSQLServer(stdout string, maxRows int) ParsedTable {
	lines := strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || sqlcmdCountLine.MatchString(trimmed) || sqlcmdRuleLine.MatchString(trimmed) {
			continue
		}
		clean = append(clean, line)
	}
	if len(clean) == 0 {
		return ParsedTable{}
	}
	joined := strings.Join(clean, "\n")
	if strings.Contains(clean[0], "\t") {
		return parseDelimited(joined, '\t', maxRows)
	}
	return ParsedTable{Error: "sqlcmd output was captured but was not tab-delimited"}
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}
