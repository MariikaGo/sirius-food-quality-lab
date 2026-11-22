package database

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io/fs"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, e
	}
	cfg.MaxConns = 20
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e == nil {
		e = p.Ping(ctx)
	}
	if e != nil && p != nil {
		p.Close()
	}
	return p, e
}
func Migrate(ctx context.Context, p *pgxpool.Pool, stage int) error {
	tx, e := p.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(813642)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name TEXT PRIMARY KEY, checksum TEXT NOT NULL)"); e != nil {
		return e
	}
	files, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range files {

		b, _ := migrations.ReadFile(name)
		sum := sha256.Sum256(b)
		checksum := hex.EncodeToString(sum[:])
		var old string
		e = tx.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE name=$1", name).Scan(&old)
		if e == nil {
			if old != checksum {
				return fmt.Errorf("migration checksum mismatch: %s", name)
			}
			continue
		}
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
		if _, e = tx.Exec(ctx, "INSERT INTO schema_migrations VALUES($1,$2)", name, checksum); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
