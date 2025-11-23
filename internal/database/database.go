package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// New создает новое подключение к PostgreSQL
func New(connString string) (*sql.DB, error) {
	const op = "database.New"

	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	// Базовые настройки пула соединений
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Простая проверка подключения
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return db, nil
}

// ConnString формирует connection string для PostgreSQL
func ConnString(host, port, user, password, dbname string) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
}

// HealthCheck проверяет доступность базы данных
func HealthCheck(db *sql.DB) error {
	return db.Ping()
}
