package db

import (
	"database/sql"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

type DB struct {
	*sql.DB
	mu sync.Mutex
}

type HistoryEntry struct {
	ID          int64     `json:"id"`
	ActionID    string    `json:"action_id"`
	Status      string    `json:"status"`
	DurationMs  int64     `json:"duration_ms"`
	LogFilePath string    `json:"log_file_path"`
	CreatedAt   time.Time `json:"created_at"`
}

func Open(dbPath string) (*DB, error) {
	d, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db := &DB{DB: d}
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		return nil, err
	}
	if err := db.init(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS history (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		action_id     TEXT     NOT NULL,
		status        TEXT     NOT NULL DEFAULT 'RUNNING',
		duration_ms   INTEGER  NOT NULL DEFAULT 0,
		log_file_path TEXT     NOT NULL,
		created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_history_created ON history(created_at DESC);
	`
	_, err := db.Exec(schema)
	return err
}

func (db *DB) InsertHistory(actionID, logPath string) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	res, err := db.Exec(
		"INSERT INTO history (action_id, log_file_path, created_at) VALUES (?, ?, datetime('now'))",
		actionID, logPath,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) UpdateHistory(id int64, status string, durationMs int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.Exec(
		"UPDATE history SET status = ?, duration_ms = ? WHERE id = ?",
		status, durationMs, id,
	)
	return err
}

func (db *DB) ListHistory(limit int) ([]HistoryEntry, error) {
	rows, err := db.Query(
		"SELECT id, action_id, status, duration_ms, log_file_path, created_at FROM history ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.ID, &e.ActionID, &e.Status, &e.DurationMs, &e.LogFilePath, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) GetHistoryByID(id int64) (*HistoryEntry, error) {
	var e HistoryEntry
	err := db.QueryRow(
		"SELECT id, action_id, status, duration_ms, log_file_path, created_at FROM history WHERE id = ?",
		id,
	).Scan(&e.ID, &e.ActionID, &e.Status, &e.DurationMs, &e.LogFilePath, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (db *DB) GetHistoryByIDs(ids []int64) ([]HistoryEntry, error) {
	if len(ids) == 0 {
		return []HistoryEntry{}, nil
	}

	query := "SELECT id, action_id, status, duration_ms, log_file_path, created_at FROM history WHERE id IN ("
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ") ORDER BY id DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.ID, &e.ActionID, &e.Status, &e.DurationMs, &e.LogFilePath, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (db *DB) DeleteHistoryBefore(t time.Time) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	res, err := db.Exec("DELETE FROM history WHERE created_at < ?", t)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
