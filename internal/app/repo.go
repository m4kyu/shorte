package app

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Close() error {
	return r.db.Close()
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *Repo) CreateUser(ctx context.Context, email, hash string) (User, error) {
	var u User
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users(email, password_hash)
		VALUES($1, $2)
		RETURNING id, email, password_hash, created_at`,
		email, hash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func (r *Repo) CreateLink(ctx context.Context, ownerID int64, code, longURL string, expiresAt *time.Time) (Link, error) {
	var l Link
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO links(code, owner_user_id, long_url, expires_at)
		VALUES($1, $2, $3, $4)
		RETURNING id, code, owner_user_id, long_url, is_active, expires_at, created_at, updated_at`,
		code, ownerID, longURL, expiresAt,
	).Scan(&l.ID, &l.Code, &l.OwnerUserID, &l.LongURL, &l.IsActive, &l.ExpiresAt, &l.CreatedAt, &l.UpdatedAt)
	return l, err
}

func (r *Repo) GetLinkByCode(ctx context.Context, code string) (Link, error) {
	var l Link
	err := r.db.QueryRowContext(ctx, `
		SELECT id, code, owner_user_id, long_url, is_active, expires_at, created_at, updated_at
		FROM links WHERE code = $1`, code).
		Scan(&l.ID, &l.Code, &l.OwnerUserID, &l.LongURL, &l.IsActive, &l.ExpiresAt, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	return l, err
}

func (r *Repo) UpdateLink(ctx context.Context, ownerID int64, code, longURL string, isActive bool, expiresAt *time.Time) (Link, error) {
	var l Link
	err := r.db.QueryRowContext(ctx, `
		UPDATE links
		SET long_url = $1, is_active = $2, expires_at = $3, updated_at = now()
		WHERE code = $4 AND owner_user_id = $5
		RETURNING id, code, owner_user_id, long_url, is_active, expires_at, created_at, updated_at`,
		longURL, isActive, expiresAt, code, ownerID).
		Scan(&l.ID, &l.Code, &l.OwnerUserID, &l.LongURL, &l.IsActive, &l.ExpiresAt, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	return l, err
}

func (r *Repo) DisableLink(ctx context.Context, ownerID int64, code string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE links
		SET is_active = false, updated_at = now()
		WHERE code = $1 AND owner_user_id = $2`,
		code, ownerID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) ListLinksByOwner(ctx context.Context, ownerID int64, limit int, offset int) ([]Link, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, code, owner_user_id, long_url, is_active, expires_at, created_at, updated_at
		FROM links
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Link, 0, limit)
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Code, &l.OwnerUserID, &l.LongURL, &l.IsActive, &l.ExpiresAt, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) UpsertDailyStat(ctx context.Context, code string, day time.Time, count int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO link_daily_stats(code, day, click_count)
		VALUES($1, $2, $3)
		ON CONFLICT (code, day)
		DO UPDATE SET click_count = link_daily_stats.click_count + EXCLUDED.click_count`,
		code, day, count)
	return err
}

func (r *Repo) GetStats(ctx context.Context, code string, from, to time.Time) ([]DailyStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT day::text, click_count
		FROM link_daily_stats
		WHERE code = $1 AND day BETWEEN $2 AND $3
		ORDER BY day ASC`, code, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyStat
	for rows.Next() {
		var s DailyStat
		if err := rows.Scan(&s.Day, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
