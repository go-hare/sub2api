package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type reputationRepository struct {
	sql sqlExecutor
}

func NewReputationRepository(sqlDB *sql.DB) service.ReputationStore {
	return &reputationRepository{sql: sqlDB}
}

func (r *reputationRepository) GetUserScore(ctx context.Context, userID int64) (int, error) {
	if r == nil || r.sql == nil {
		return service.ReputationMinScore, errors.New("reputation repository unavailable")
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT reputation_score FROM users WHERE id = $1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, sql.ErrNoRows
	}
	var score int
	if err := rows.Scan(&score); err != nil {
		return 0, err
	}
	return service.ClampReputationScore(score), nil
}

func (r *reputationRepository) ApplyUserScoreChange(ctx context.Context, userID int64, delta int, absolute *int) (int, int, error) {
	if r == nil || r.sql == nil {
		return 0, 0, errors.New("reputation repository unavailable")
	}
	var (
		query string
		args  []any
	)
	if absolute != nil {
		query = `
WITH prev AS (
  SELECT id, reputation_score
  FROM users
  WHERE id = $1 AND deleted_at IS NULL
  FOR UPDATE
)
UPDATE users u
SET reputation_score = GREATEST($2, LEAST($3, $4)),
    updated_at = NOW()
FROM prev
WHERE u.id = prev.id
RETURNING prev.reputation_score, u.reputation_score`
		args = []any{userID, service.ReputationMinScore, service.ReputationMaxScore, service.ClampReputationScore(*absolute)}
	} else {
		query = `
WITH prev AS (
  SELECT id, reputation_score
  FROM users
  WHERE id = $1 AND deleted_at IS NULL
  FOR UPDATE
)
UPDATE users u
SET reputation_score = GREATEST($2, LEAST($3, prev.reputation_score + $4)),
    updated_at = NOW()
FROM prev
WHERE u.id = prev.id
RETURNING prev.reputation_score, u.reputation_score`
		args = []any{userID, service.ReputationMinScore, service.ReputationMaxScore, delta}
	}
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, 0, service.ErrUserNotFound
	}
	var before, after int
	if err := rows.Scan(&before, &after); err != nil {
		return 0, 0, err
	}
	return service.ClampReputationScore(before), service.ClampReputationScore(after), nil
}

func (r *reputationRepository) InsertLog(ctx context.Context, log service.ReputationScoreLog) error {
	if r == nil || r.sql == nil {
		return errors.New("reputation repository unavailable")
	}
	_, err := r.sql.ExecContext(ctx, `
INSERT INTO reputation_score_logs (user_id, event, delta, score_before, score_after, request_id, account_id, amount, actor_admin_id, reason)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		log.UserID, log.Event, log.Delta, log.ScoreBefore, log.ScoreAfter, log.RequestID, log.AccountID, log.Amount, log.ActorAdminID, log.Reason)
	return err
}

func (r *reputationRepository) CountAccountsByTierID(ctx context.Context, tierID string) (int64, error) {
	if r == nil || r.sql == nil {
		return 0, errors.New("reputation repository unavailable")
	}
	rows, err := r.sql.QueryContext(ctx, `
SELECT COUNT(*) FROM accounts WHERE reputation_tier_id = $1 AND deleted_at IS NULL`, tierID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, nil
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *reputationRepository) SetAccountReputation(ctx context.Context, accountID int64, patch service.AccountReputationPatch) error {
	if r == nil || r.sql == nil {
		return errors.New("reputation repository unavailable")
	}
	sets := make([]string, 0, 3)
	args := []any{accountID}
	n := 2
	if patch.TierID != nil {
		var tier any
		if strings.TrimSpace(*patch.TierID) != "" {
			tier = strings.TrimSpace(*patch.TierID)
		}
		sets = append(sets, fmt.Sprintf("reputation_tier_id = $%d", n))
		args = append(args, tier)
		n++
	}
	if patch.SetScoreEnabled {
		sets = append(sets, fmt.Sprintf("reputation_score_enabled = $%d", n))
		args = append(args, patch.ScoreEnabled)
		n++
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = NOW()")
	_, err := r.sql.ExecContext(ctx, fmt.Sprintf(`UPDATE accounts SET %s WHERE id = $1 AND deleted_at IS NULL`, strings.Join(sets, ", ")), args...)
	return err
}

func attachUserReputationScores(ctx context.Context, exec sqlExecutor, users []service.User) {
	if exec == nil || len(users) == 0 {
		return
	}
	ids := make([]int64, 0, len(users))
	index := make(map[int64]int, len(users))
	for i := range users {
		ids = append(ids, users[i].ID)
		index[users[i].ID] = i
	}
	rows, err := exec.QueryContext(ctx, `SELECT id, reputation_score FROM users WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var score int
		if err := rows.Scan(&id, &score); err != nil {
			return
		}
		if i, ok := index[id]; ok {
			users[i].ReputationScore = service.ClampReputationScore(score)
		}
	}
}

func attachAccountReputation(ctx context.Context, exec sqlExecutor, accounts []service.Account) {
	if exec == nil || len(accounts) == 0 {
		return
	}
	ids := make([]int64, 0, len(accounts))
	index := make(map[int64]int, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
		index[accounts[i].ID] = i
	}
	rows, err := exec.QueryContext(ctx, `
SELECT id, reputation_tier_id, reputation_score_enabled
FROM accounts WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var tierID sql.NullString
		var enabled sql.NullBool
		if err := rows.Scan(&id, &tierID, &enabled); err != nil {
			return
		}
		i, ok := index[id]
		if !ok {
			continue
		}
		if tierID.Valid {
			accounts[i].ReputationTierID = tierID.String
		}
		if enabled.Valid {
			value := enabled.Bool
			accounts[i].ReputationScoreEnabled = &value
		}
	}
}

func attachOneUserReputation(ctx context.Context, exec sqlExecutor, user *service.User) {
	if user == nil {
		return
	}
	users := []service.User{*user}
	attachUserReputationScores(ctx, exec, users)
	*user = users[0]
}

func attachOneAccountReputation(ctx context.Context, exec sqlExecutor, account *service.Account) {
	if account == nil {
		return
	}
	accounts := []service.Account{*account}
	attachAccountReputation(ctx, exec, accounts)
	*account = accounts[0]
}

func persistAccountReputation(ctx context.Context, exec sqlExecutor, account *service.Account) error {
	if exec == nil || account == nil || account.ID == 0 {
		return nil
	}
	var tier any
	if account.ReputationTierID != "" {
		tier = account.ReputationTierID
	}
	_, err := exec.ExecContext(ctx, `
UPDATE accounts SET reputation_tier_id = $2, reputation_score_enabled = $3 WHERE id = $1`,
		account.ID, tier, account.ReputationScoreEnabled)
	if err != nil {
		return fmt.Errorf("persist account reputation: %w", err)
	}
	return nil
}
