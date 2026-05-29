# clankerwatch (agents)

Humans use the web UI; agents use the `cwatch` CLI.

## Quick start

1. Ensure the app is running (`cwatch` from repo root, or ask the human to start it).

2. List profiles and pick one the human configured:

```powershell
cwatch profile list
```

3. Pick a short investigation slug once per Cursor chat (lowercase letters, numbers, hyphens):

```powershell
cwatch query <profile> --session storefront-audit --reason "why this query matters" --sql "select ...;"
```

4. After each useful query, annotate what you learned:

```powershell
cwatch annotate --session storefront-audit --note "what you learned"
cwatch annotate --session storefront-audit --rows 4-8 --note "why these rows matter"
```

Use the same `--session` slug for every command in one investigation. Subagents should reuse the slug from the task.

## Full reference

- Human + agent install and examples: [README.md](README.md)
- Agent workflow rules and command cheatsheet: [skills/clankerwatch/SKILL.md](skills/clankerwatch/SKILL.md)

## Rules

- Never ask for database credentials; use configured profiles only.
- Every query and annotate needs `--session`.
- Every query needs `--reason`.
- Annotate conclusions after each useful query.
- Use `--rows` when annotating specific rows; those rows are highlighted in the UI for the human.

## Local paths

| What | Default |
|------|---------|
| Audit DB | `%AppData%\clankerwatch\clankerwatch.sqlite` |
| Server file | `%LocalAppData%\clankerwatch\session.json` |
| Override data | `$env:CWATCH_DATA_DIR` |
| Override cache | `$env:CWATCH_CACHE_DIR` |

## Verify (maintainers)

```powershell
go test ./...
bun run build
```
