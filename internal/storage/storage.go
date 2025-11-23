package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"review-assignment/internal/models"

	_ "github.com/lib/pq"
)

var (
	ErrTeamExists     = errors.New("TEAM_EXISTS")
	ErrPRExists       = errors.New("PR_EXISTS")
	ErrPRMerged       = errors.New("PR_MERGED")
	ErrNotFound       = errors.New("NOT_FOUND")
	ErrNotAssigned    = errors.New("NOT_ASSIGNED")
	ErrNoCandidate    = errors.New("NO_CANDIDATE")
	ErrAuthorNotFound = errors.New("AUTHOR_NOT_FOUND")
)

type Storage struct {
	db  *sql.DB
	rng *rand.Rand
}

func New(db *sql.DB) *Storage {
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)

	return &Storage{
		db:  db,
		rng: rng,
	}
}

// TEAM METHODS

func (s *Storage) CreateTeam(team models.Team) error {
	const op = "storage.CreateTeam"

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
        INSERT INTO teams (name) VALUES ($1)
    `, team.Name)
	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"teams_pkey\"" {
			return fmt.Errorf("%s: %w", op, ErrTeamExists)
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	for _, member := range team.Members {
		_, err := tx.Exec(`
            INSERT INTO users (user_id, username, team_name, is_active) 
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (user_id) DO UPDATE SET 
                username = EXCLUDED.username, 
                team_name = EXCLUDED.team_name, 
                is_active = EXCLUDED.is_active,
                updated_at = NOW()
        `, member.ID, member.Username, team.Name, member.IsActive)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	return tx.Commit()
}

func (s *Storage) GetTeam(teamName string) (*models.Team, error) {
	const op = "storage.GetTeam"

	var teamExists bool
	err := s.db.QueryRow(`
        SELECT EXISTS(SELECT 1 FROM teams WHERE name = $1)
    `, teamName).Scan(&teamExists)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if !teamExists {
		return nil, fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	rows, err := s.db.Query(`
        SELECT user_id, username, is_active 
        FROM users 
        WHERE team_name = $1
        ORDER BY user_id
    `, teamName)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var members []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.IsActive); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		members = append(members, user)
	}

	return &models.Team{
		Name:    teamName,
		Members: members,
	}, nil
}

// USER METHODS

func (s *Storage) SetUserActive(userID string, isActive bool) (*models.User, error) {
	const op = "storage.SetUserActive"

	var user models.User
	err := s.db.QueryRow(`
		UPDATE users 
		SET is_active = $1, updated_at = NOW() 
		WHERE user_id = $2
		RETURNING user_id, username, team_name, is_active
	`, isActive, userID).Scan(&user.ID, &user.Username, &user.TeamName, &user.IsActive)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &user, nil
}

func (s *Storage) GetUserReviews(userID string) ([]models.PullRequest, error) {
	const op = "storage.GetUserReviews"

	var userExists bool
	err := s.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)
	`, userID).Scan(&userExists)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if !userExists {
		return nil, fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	rows, err := s.db.Query(`
		SELECT 
			pr.pull_request_id, pr.pull_request_name, 
			pr.author_id, pr.status, pr.created_at, pr.merged_at
		FROM pull_requests pr
		JOIN pr_reviewers prr ON pr.pull_request_id = prr.pr_id
		WHERE prr.reviewer_id = $1
		ORDER BY pr.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var prs []models.PullRequest
	for rows.Next() {
		var pr models.PullRequest
		var statusStr string
		var mergedAt sql.NullTime

		if err := rows.Scan(
			&pr.ID, &pr.Name, &pr.AuthorID, &statusStr,
			&pr.CreatedAt, &mergedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		pr.Status = models.PRStatus(statusStr)
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}

		prs = append(prs, pr)
	}

	return prs, nil
}

// PR METHODS

func (s *Storage) CreatePR(req models.CreatePRRequest) (*models.PullRequest, error) {
	const op = "storage.CreatePR"

	var author models.User
	err := s.db.QueryRow(`
		SELECT user_id, username, team_name, is_active 
		FROM users WHERE user_id = $1
	`, req.AuthorID).Scan(&author.ID, &author.Username, &author.TeamName, &author.IsActive)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: %w", op, ErrAuthorNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	rows, err := s.db.Query(`
		SELECT user_id 
		FROM users 
		WHERE team_name = $1 AND is_active = true AND user_id != $2
	`, author.TeamName, author.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var candidateIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		candidateIDs = append(candidateIDs, userID)
	}

	reviewers := s.selectRandomReviewers(candidateIDs, 2)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	var prExists bool
	err = tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)
	`, req.ID).Scan(&prExists)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if prExists {
		return nil, fmt.Errorf("%s: %w", op, ErrPRExists)
	}

	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, req.ID, req.Name, req.AuthorID, models.StatusOpen, now)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	for _, reviewer := range reviewers {
		_, err := tx.Exec(`
			INSERT INTO pr_reviewers (pr_id, reviewer_id) VALUES ($1, $2)
		`, req.ID, reviewer)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &models.PullRequest{
		ID:                req.ID,
		Name:              req.Name,
		AuthorID:          req.AuthorID,
		Status:            models.StatusOpen,
		AssignedReviewers: reviewers,
		CreatedAt:         now,
	}, nil
}

func (s *Storage) MergePR(prID string) (*models.PullRequest, error) {
	const op = "storage.MergePR"

	pr, err := s.getPRWithReviewers(prID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if pr.Status == models.StatusMerged {
		return pr, nil
	}

	now := time.Now()
	_, err = s.db.Exec(`
		UPDATE pull_requests 
		SET status = $1, merged_at = $2 
		WHERE pull_request_id = $3
	`, models.StatusMerged, now, prID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	pr.Status = models.StatusMerged
	pr.MergedAt = &now
	return pr, nil
}

func (s *Storage) ReassignReviewer(req models.ReassignRequest) (*models.PullRequest, string, error) {
	const op = "storage.ReassignReviewer"

	pr, err := s.getPRWithReviewers(req.PRID)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}

	if pr.Status == models.StatusMerged {
		return nil, "", fmt.Errorf("%s: %w", op, ErrPRMerged)
	}

	if !s.contains(pr.AssignedReviewers, req.OldReviewer) {
		return nil, "", fmt.Errorf("%s: %w", op, ErrNotAssigned)
	}

	var oldReviewerTeam string
	err = s.db.QueryRow(`
		SELECT team_name FROM users WHERE user_id = $1
	`, req.OldReviewer).Scan(&oldReviewerTeam)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}

	candidateIDs, err := s.findReplacementCandidates(oldReviewerTeam, pr.AuthorID, pr.AssignedReviewers, req.OldReviewer)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}

	if len(candidateIDs) == 0 {
		return nil, "", fmt.Errorf("%s: %w", op, ErrNoCandidate)
	}

	newReviewer := s.selectRandomReviewer(candidateIDs)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		DELETE FROM pr_reviewers 
		WHERE pr_id = $1 AND reviewer_id = $2
	`, req.PRID, req.OldReviewer)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}

	_, err = tx.Exec(`
		INSERT INTO pr_reviewers (pr_id, reviewer_id) VALUES ($1, $2)
	`, req.PRID, newReviewer)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, err)
	}

	newReviewers := s.replaceInSlice(pr.AssignedReviewers, req.OldReviewer, newReviewer)
	pr.AssignedReviewers = newReviewers

	return pr, newReviewer, nil
}

// HELPER METHODS

func (s *Storage) getPRWithReviewers(prID string) (*models.PullRequest, error) {
	const op = "storage.getPRWithReviewers"

	var pr models.PullRequest
	var statusStr string
	var mergedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT 
			pull_request_id, pull_request_name, author_id, status, 
			created_at, merged_at
		FROM pull_requests 
		WHERE pull_request_id = $1
	`, prID).Scan(
		&pr.ID, &pr.Name, &pr.AuthorID, &statusStr,
		&pr.CreatedAt, &mergedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	pr.Status = models.PRStatus(statusStr)
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}

	reviewers, err := s.getPRReviewers(prID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	pr.AssignedReviewers = reviewers

	return &pr, nil
}

func (s *Storage) getPRReviewers(prID string) ([]string, error) {
	const op = "storage.getPRReviewers"

	rows, err := s.db.Query(`
		SELECT reviewer_id 
		FROM pr_reviewers 
		WHERE pr_id = $1
		ORDER BY assigned_at
	`, prID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var reviewers []string
	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		reviewers = append(reviewers, reviewerID)
	}

	return reviewers, nil
}

func (s *Storage) findReplacementCandidates(teamName, authorID string, currentReviewers []string, excludeReviewer string) ([]string, error) {
	query := `
		SELECT user_id 
		FROM users 
		WHERE team_name = $1 
		AND is_active = true 
		AND user_id != $2 
		AND user_id != $3
	`

	params := []interface{}{teamName, authorID, excludeReviewer}
	paramCount := 4

	for _, reviewer := range currentReviewers {
		if reviewer != excludeReviewer {
			query += fmt.Sprintf(" AND user_id != $%d", paramCount)
			params = append(params, reviewer)
			paramCount++
		}
	}

	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		candidates = append(candidates, userID)
	}

	return candidates, nil
}

func (s *Storage) selectRandomReviewers(candidates []string, max int) []string {
	if len(candidates) == 0 {
		return []string{}
	}

	shuffled := make([]string, len(candidates))
	copy(shuffled, candidates)

	s.rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	if len(shuffled) <= max {
		return shuffled
	}
	return shuffled[:max]
}

func (s *Storage) selectRandomReviewer(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	return candidates[s.rng.Intn(len(candidates))]
}

func (s *Storage) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (s *Storage) replaceInSlice(slice []string, old, new string) []string {
	result := make([]string, len(slice))
	for i, item := range slice {
		if item == old {
			result[i] = new
		} else {
			result[i] = item
		}
	}
	return result
}
