package auth

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	return nil
}

func (s *Store) EnsureAdmin(user, passwordHash string) error {
	count := 0
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		user,
		passwordHash,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) GetPasswordHash(user string) (string, bool) {
	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, user).Scan(&hash)
	if err != nil {
		return "", false
	}
	return hash, true
}

func (s *Store) SaveSession(token, user string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO sessions (token, username, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		token,
		user,
		expiresAt.UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) ValidateSession(token string) (bool, error) {
	var expires string
	err := s.db.QueryRow(`SELECT expires_at FROM sessions WHERE token = ?`, token).Scan(&expires)
	if err != nil {
		return false, nil
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return false, err
	}
	return time.Now().UTC().Before(expiresAt), nil
}
