package app

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Worker struct {
	repo  *Repo
	cache *Cache
}

func NewWorker(cfg Config) (*Worker, error) {
	db, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Worker{
		repo:  NewRepo(db),
		cache: NewCache(cfg),
	}, nil
}

func (w *Worker) Close() {
	_ = w.repo.Close()
	_ = w.cache.Close()
}

func (w *Worker) Run(ctx context.Context) error {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := w.flush(ctx); err != nil {
				// best effort loop
			}
		}
	}
}

func (w *Worker) flush(ctx context.Context) error {
	items, err := w.cache.PopClicks(ctx, 500)
	if err != nil || len(items) == 0 {
		return err
	}
	agg := map[string]map[time.Time]int64{}
	for _, it := range items {
		p := strings.Split(it, "|")
		if len(p) != 2 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, p[1])
		if err != nil {
			continue
		}
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
		if agg[p[0]] == nil {
			agg[p[0]] = map[time.Time]int64{}
		}
		agg[p[0]][day]++
	}
	for code, m := range agg {
		for day, c := range m {
			if err := w.repo.UpsertDailyStat(ctx, code, day, c); err != nil {
				return err
			}
		}
	}
	return nil
}

