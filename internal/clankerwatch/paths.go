package clankerwatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type SessionInfo struct {
	APIBase   string    `json:"apiBase"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"createdAt"`
}

func dataDir() (string, error) {
	if dir := os.Getenv("CWATCH_DATA_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "clankerwatch"), nil
}

func cacheDir() (string, error) {
	if dir := os.Getenv("CWATCH_CACHE_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "clankerwatch"), nil
}

func dbPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "clankerwatch.sqlite"), nil
}

func sessionPath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

func writeSession(info SessionInfo) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readSession() (SessionInfo, error) {
	path, err := sessionPath()
	if err != nil {
		return SessionInfo{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SessionInfo{}, errors.New("clankerwatch server is not running; start it with `cwatch` first")
		}
		return SessionInfo{}, err
	}
	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return SessionInfo{}, err
	}
	if info.APIBase == "" {
		return SessionInfo{}, errors.New("clankerwatch session file is invalid; restart `cwatch`")
	}
	return info, nil
}
