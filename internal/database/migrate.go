package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string, direction Direction) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %q: %w", dir, err)
	}

	suffix := "." + string(direction) + ".sql"
	versions := make([]int64, 0, len(files))
	byVersion := make(map[int64]string)
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), suffix) {
			continue
		}
		base := strings.TrimSuffix(f.Name(), suffix)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) == 0 {
			continue
		}
		v, perr := strconv.ParseInt(parts[0], 10, 64)
		if perr != nil {
			continue
		}
		versions = append(versions, v)
		byVersion[v] = filepath.Join(dir, f.Name())
	}

	if direction == DirectionUp {
		sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	} else {
		sort.Slice(versions, func(i, j int) bool { return versions[i] > versions[j] })
	}

	applied, err := loadApplied(ctx, pool)
	if err != nil {
		return err
	}

	for _, v := range versions {
		switch direction {
		case DirectionUp:
			if _, ok := applied[v]; ok {
				continue
			}
		case DirectionDown:
			if _, ok := applied[v]; !ok {
				continue
			}
		}
		sqlBytes, err := os.ReadFile(byVersion[v])
		if err != nil {
			return fmt.Errorf("read %s: %w", byVersion[v], err)
		}
		log.Info().Int64("version", v).Str("direction", string(direction)).Msg("applying migration")
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %d: %w", v, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %d: %w", v, err)
		}
		switch direction {
		case DirectionUp:
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING`, v); err != nil {
				_ = tx.Rollback(ctx)
				return fmt.Errorf("record migration %d: %w", v, err)
			}
		case DirectionDown:
			if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, v); err != nil {
				_ = tx.Rollback(ctx)
				return fmt.Errorf("unrecord migration %d: %w", v, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", v, err)
		}
		log.Info().Int64("version", v).Msg("migration applied")
	}
	return nil
}

func loadApplied(ctx context.Context, pool *pgxpool.Pool) (map[int64]struct{}, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied: %w", err)
	}
	defer rows.Close()
	applied := make(map[int64]struct{})
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = struct{}{}
	}
	return applied, rows.Err()
}
