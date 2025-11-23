package storage

import (
	"fmt"
)

func (s *Storage) Init() error {
	const op = "storage.Init"

	_, err := s.db.Exec(`
        CREATE TABLE IF NOT EXISTS teams (
            name VARCHAR(100) PRIMARY KEY,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )
    `)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = s.db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            user_id VARCHAR(50) PRIMARY KEY,
            username VARCHAR(100) NOT NULL,
            team_name VARCHAR(100) NOT NULL REFERENCES teams(name) ON DELETE CASCADE,
            is_active BOOLEAN DEFAULT TRUE,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )
    `)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS pull_requests (
			pull_request_id VARCHAR(50) PRIMARY KEY,
			pull_request_name VARCHAR(200) NOT NULL,
			author_id VARCHAR(50) NOT NULL REFERENCES users(user_id),
			status VARCHAR(20) DEFAULT 'OPEN',
			created_at TIMESTAMP DEFAULT NOW(),
			merged_at TIMESTAMP NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS pr_reviewers (
			pr_id VARCHAR(50) REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
			reviewer_id VARCHAR(50) REFERENCES users(user_id),
			assigned_at TIMESTAMP DEFAULT NOW(),
			PRIMARY KEY (pr_id, reviewer_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	// Создаём индексы
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_users_team ON users(team_name);
		CREATE INDEX IF NOT EXISTS idx_users_active ON users(team_name, is_active);
		CREATE INDEX IF NOT EXISTS idx_pr_reviewers ON pr_reviewers(reviewer_id);
		CREATE INDEX IF NOT EXISTS idx_pr_status ON pull_requests(status);
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
