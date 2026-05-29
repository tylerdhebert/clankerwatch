# clankerwatch

`clankerwatch` is a local database query watcher for agent-driven debugging. The human configures database CLI profiles in the browser, and agents use the `cwatch` CLI to run queries and annotate what they learned.

The app serves at `https://cwatch.localhost` through Vercel Portless. The Go API serves at `https://cwatchapi.localhost`.

## Install

### humans

One-command setup (requires [Go](https://go.dev/dl/) and [Bun](https://bun.sh/)):

```powershell
bun run setup
```

This installs dependencies, links `cwatch`, and builds the CLI and web app.

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

Run queries through configured profiles (pick one `--session` slug per investigation):

```powershell
cwatch query effortless-sqlite --session effort-audit --reason "checking active efforts" --sql "select id, short_ref, status from efforts;"
```

Annotate what you learned (targets latest run in that session by default):

```powershell
cwatch annotate --session effort-audit --note "The active effort count matches the dashboard."
cwatch annotate --session effort-audit --rows 3-7 --note "These rows share the same status transition."
```

## CLI

```powershell
cwatch profile list
cwatch profile show effortless-sqlite
cwatch query <profile> --session <slug> --reason <text> --sql <sql>
cwatch query <profile> --session <slug> --reason <text> --file .\query.sql
Get-Content .\query.sql | cwatch query <profile> --session <slug> --reason <text> --stdin
cwatch annotate --session <slug> [<run-id>] --note <text> [--rows <n-m>]
```

By default, `query` prints a numbered table with a run id footer. Add `--json` when structured metadata is needed.

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
