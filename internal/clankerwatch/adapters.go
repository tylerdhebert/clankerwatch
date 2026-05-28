package clankerwatch

import (
	"strings"
)

type CommandSpec struct {
	Name string
	Args []string
	Env  map[string]string
}

func BuildCommand(profile Profile, queryFile string, secretEnv map[string]string) CommandSpec {
	command := strings.TrimSpace(profile.Command)
	if command == "" {
		command = defaultCommand(profile.Adapter)
	}

	args := expandArgs(profile.Args, queryFile)
	if !argsContainQueryFile(profile.Args) {
		args = addDefaultQueryArgs(profile.Adapter, args, queryFile)
	}

	env := map[string]string{}
	for key, value := range profile.Env {
		env[key] = value
	}
	for key, value := range secretEnv {
		env[key] = value
	}

	return CommandSpec{Name: command, Args: args, Env: env}
}

func defaultCommand(adapter string) string {
	switch adapter {
	case "postgres":
		return "psql"
	case "sqlite":
		return "sqlite3"
	case "sqlserver":
		return "sqlcmd"
	default:
		return ""
	}
}

func addDefaultQueryArgs(adapter string, args []string, queryFile string) []string {
	switch adapter {
	case "postgres":
		return append(args, "--csv", "--file", queryFile)
	case "sqlite":
		next := []string{"-readonly", "-cmd", ".headers on", "-cmd", ".mode csv"}
		next = append(next, args...)
		return append(next, ".read '"+sqlitePath(queryFile)+"'")
	case "sqlserver":
		return append(args, "-W", "-s", "\t", "-i", queryFile)
	default:
		return args
	}
}

func sqlitePath(path string) string {
	return strings.ReplaceAll(strings.ReplaceAll(path, `\`, `/`), `'`, `''`)
}

func expandArgs(args []string, queryFile string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, strings.ReplaceAll(arg, "{query_file}", queryFile))
	}
	return out
}

func argsContainQueryFile(args []string) bool {
	for _, arg := range args {
		if strings.Contains(arg, "{query_file}") {
			return true
		}
	}
	return false
}
