---
name: clankerwatch
description: Use clankerwatch's cwatch CLI to run database queries through human-configured local profiles while keeping credentials out of agent context.
---

# clankerwatch

Use `cwatch` when you need to inspect a database through a profile the human configured in the clankerwatch web app.

## Start

Confirm the server is up:

```powershell
cwatch profile list
```

If it fails, ask the human to start the app with `cwatch` in the repo root.

Pick one investigation slug per chat (letters, numbers, hyphens; stored lowercase):

```text
storefront-audit
```

Reuse that slug on every command. Pass it to subagents in the task text.

## Query

Run SQL with a clear reason:

```powershell
cwatch query <profile> --session storefront-audit --reason "why this query matters" --sql "select ...;"
```

For longer SQL:

```powershell
cwatch query <profile> --session storefront-audit --reason-file .\reason.txt --file .\query.sql
```

Output is a numbered table with a run id footer. Use `--json` when you need structured metadata.

## Annotate

Record what you learned (targets the latest run in that session by default):

```powershell
cwatch annotate --session storefront-audit --note "what this query showed"
```

Annotate specific rows or ranges (highlights them in the UI):

```powershell
cwatch annotate --session storefront-audit --row 4 --note "why this row matters"
cwatch annotate --session storefront-audit --rows 4-8 --note "why this range matters"
```

To annotate a specific run instead of the latest:

```powershell
cwatch annotate --session storefront-audit 42 --note "what this showed"
```

## Rules

- Use only configured profiles.
- Pass `--session` on every query and annotate.
- Write a real `--reason` for every query.
- Do not ask the human for database credentials.
- Annotate conclusions after each useful query.
- Use `--rows` when the human should inspect specific result rows.
