# clankerwatch

`clankerwatch` is a local database query watcher for agent-driven debugging. The human configures database CLI profiles in the browser, and agents use the `cwatch` CLI to run queries, annotate what they learned, and highlight result rows.

The app serves at `https://cwatch.localhost` through Vercel Portless. The Go API serves at `https://cwatchapi.localhost`.

## Install

### humans

Install dependencies and link the command:

```powershell
bun install
Push-Location web
bun install
Pop-Location
bun link
```

Start the app:

```powershell
cwatch
```

Open the profile modal from the top-right `profiles` button. Create a profile, enter command args, and unlock secret env values in memory.

Portless may ask to trust its local CA on first run. Accept that prompt to use `https://cwatch.localhost` and `https://cwatchapi.localhost`.

For raw fixed-port dev without Portless:

```powershell
bun run dev:raw
```

Raw mode uses `http://127.0.0.1:5173` for Vue and `http://127.0.0.1:48731` for the Go API.

### agents

Start a shell-scoped clankerwatch session:

```powershell
cwatch session create --name "short task name" | iex
```

Run queries through configured profiles:

```powershell
cwatch query effortless-sqlite --reason "checking active efforts" --sql "select id, short_ref, status from efforts;"
```

Annotate what you learned:

```powershell
cwatch annotate 12 --note "The active effort count matches the dashboard."
cwatch annotate 12 --rows 3-7 --note "These rows share the same status transition."
```

Highlight rows for the human:

```powershell
cwatch highlight 12 --row 4 --note "This row has the unexpected state."
cwatch highlight 12 --rows 3-7 --note "Inspect this contiguous range."
```

If the shell loses its session env:

```powershell
cwatch session reattach latest | iex
```

## CLI

```powershell
cwatch status
cwatch profile list
cwatch profile show effortless-sqlite
cwatch session list
cwatch query <profile> --reason <text> --sql <sql>
cwatch query <profile> --reason <text> --file .\query.sql
Get-Content .\query.sql | cwatch query <profile> --reason <text> --stdin
cwatch annotate <run-id> --note <text>
cwatch highlight <run-id> --rows 1-3 --note <text>
```

By default, `query` prints the wrapped database CLI stdout. Add `--json` when structured run metadata is useful.

## Profile Examples

SQLite:

```text
adapter: sqlite
command: sqlite3
args:
  C:\path\to\app.db
```

Postgres:

```text
adapter: postgres
command: psql
secret env:
  DATABASE_URL=postgres://...
```

SQL Server:

```text
adapter: sqlserver
command: sqlcmd
args:
  -S
  server-name
  -d
  database-name
  -U
  readonly-user
secret env:
  SQLCMDPASSWORD=...
```

Generic CSV command:

```text
adapter: generic
command: powershell
args:
  -NoProfile
  -Command
  "Write-Output 'id,name'; Write-Output '1,example'"
```

## Local Data

Audit DB:

```text
%AppData%\clankerwatch\clankerwatch.sqlite
```

Server metadata:

```text
%LocalAppData%\clankerwatch\session.json
```

Temporary override paths:

```powershell
$env:CWATCH_DATA_DIR = "C:\temp\clankerwatch-data"
$env:CWATCH_CACHE_DIR = "C:\temp\clankerwatch-cache"
```

Suppress browser opening:

```powershell
$env:CWATCH_NO_OPEN = "1"
```

## Agents

Agents should read [AGENTS.md](AGENTS.md) first, then the bundled skill:

```text
skills\clankerwatch\SKILL.md
```

Copy or symlink `skills\clankerwatch` into your agent skills directory if your tooling expects skills on disk.

## Verification

```powershell
go test ./...
bun run build
```

`bun run build` compiles the Go CLI and the Vue app.
