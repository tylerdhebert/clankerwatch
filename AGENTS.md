# clankerwatch (agents)

Humans use the web UI; agents use the `cwatch` CLI.

## Quick start

1. Ensure the app is running (`cwatch` from repo root, or ask the human to start it).
2. Create a shell session (PowerShell):

```powershell
cwatch session create --name "short task name" | iex
```

3. List profiles and pick one the human configured:

```powershell
cwatch profile list
```

4. Run read-only queries with a real reason:

```powershell
cwatch query <profile> --reason "why this query matters" --sql "select ...;"
```

5. After each useful query, annotate and highlight rows for the human:

```powershell
cwatch annotate <run-id> --note "what you learned"
cwatch highlight <run-id> --row 4 --note "inspect this row"
```

If the shell lost session env:

```powershell
cwatch session reattach latest | iex
```

Check server health:

```powershell
cwatch status
```

## Full reference

- Human + agent install and examples: [README.md](README.md)
- Agent workflow rules and command cheatsheet: [skills/clankerwatch/SKILL.md](skills/clankerwatch/SKILL.md)

## Rules

- Never ask for database credentials; use configured profiles only.
- Every query needs `--reason` (or `--reason-file`).
- Prefer focused read-only SQL; the server blocks obvious writes.
- Default `query` output is the underlying CLI stdout; add `--json` only when you need run metadata.

## Local paths

| What | Default |
|------|---------|
| Audit DB | `%AppData%\clankerwatch\clankerwatch.sqlite` |
| Session file | `%LocalAppData%\clankerwatch\session.json` |
| Override data | `$env:CWATCH_DATA_DIR` |
| Override cache | `$env:CWATCH_CACHE_DIR` |

## Verify (maintainers)

```powershell
go test ./...
bun run build
```
