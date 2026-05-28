---
name: clankerwatch
description: Use clankerwatch's cwatch CLI to run database queries through human-configured local profiles while keeping credentials out of agent context.
---

# clankerwatch

Use `cwatch` when you need to inspect a database through a profile the human configured in the clankerwatch web app.

## Start

Confirm the server is up:

```powershell
cwatch status
```

Create or reattach a shell session before running queries:

```powershell
cwatch session create --name "short task name" | iex
```

If the shell lost its session:

```powershell
cwatch session reattach latest | iex
```

Check available profiles:

```powershell
cwatch profile list
```

## Query

Run read-only SQL with a clear reason:

```powershell
cwatch query <profile> --reason "why this query matters" --sql "select ...;"
```

For longer SQL:

```powershell
cwatch query <profile> --reason-file .\reason.txt --file .\query.sql
```

Default output is the underlying database CLI stdout. Use `--json` only when you need the run id or structured metadata.

## Annotate

Record what you learned:

```powershell
cwatch annotate <run-id> --note "what this query showed"
```

Attach notes to rows or ranges:

```powershell
cwatch annotate <run-id> --row 4 --note "why this row matters"
cwatch annotate <run-id> --rows 4-8 --note "why this range matters"
```

## Highlight

Point the human at important rows:

```powershell
cwatch highlight <run-id> --row 4 --note "inspect this row"
cwatch highlight <run-id> --rows 4-8 --note "inspect this range"
```

## Rules

- Use only configured profiles.
- Write a real `--reason` for every query.
- Prefer focused, read-only SQL.
- Do not ask the human for database credentials.
- Annotate conclusions after each useful query.
- Highlight rows or row ranges when the human should inspect specific results.
