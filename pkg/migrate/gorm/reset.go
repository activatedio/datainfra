package gorm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
)

// TableResetOption configures NewTableReset.
type TableResetOption func(*tableReset)

// WithTables names the tables to clear, in the order to clear them. Without
// it the reset introspects the database and clears every table except goose
// version tables, deferring any DELETE that trips a foreign key to a later
// pass until the set is empty or no pass makes progress.
func WithTables(tables ...string) TableResetOption {
	return func(r *tableReset) {
		r.tables = tables
	}
}

// WithKeep names tables the reset must leave alone, on top of the goose
// version tables it always keeps. Use it for a base-tier data table that a
// ReuseWithMigrate delta must not wipe.
func WithKeep(tables ...string) TableResetOption {
	return func(r *tableReset) {
		for _, t := range tables {
			r.keep[t] = true
		}
	}
}

type tableReset struct {
	config *datagorm.Config
	tables []string
	keep   map[string]bool
}

// NewTableReset returns a Reversible whose Up is a no-op and whose Down
// deletes every row from every data table. It is the generic undo for a
// delta tier whose Up is a set of one-directional migrators — bootstrap
// loaders, seeders — sitting on a schema-only base: rather than each loader
// authoring the reverse of every row it wrote, the sequence ends in a reset.
//
// Rows are removed with DELETE, never TRUNCATE or DROP: on YugabyteDB DDL is
// two orders of magnitude slower than DML and bumps every session's catalog
// version, which is the cost this whole lifecycle exists to avoid.
//
// A reset is only correct on a base that carries no data of its own. Pair it
// with WithKeep when the base does.
func NewTableReset(config *MigratorGormConfig, opts ...TableResetOption) migrate.Reversible {
	r := &tableReset{config: &config.GormConfig, keep: map[string]bool{}}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *tableReset) Up(_ context.Context) error { return nil }

func (r *tableReset) Down(ctx context.Context) error {
	start := time.Now()
	gdb, err := datagorm.NewDB(r.config)
	if err != nil {
		return err
	}
	defer func() {
		if sdb, derr := gdb.DB(); derr == nil {
			_ = sdb.Close()
		}
	}()
	gdb = gdb.WithContext(ctx)

	tables, err := r.targets(gdb)
	if err != nil {
		return err
	}

	// Multi-pass: a DELETE that fails on a foreign key is retried after the
	// tables that reference it have been cleared. Each statement runs in its
	// own implicit transaction, so a failure aborts nothing but itself.
	remaining := tables
	var deleted int
	for pass := 1; len(remaining) > 0; pass++ {
		var deferred []string
		var lastErr error
		for _, t := range remaining {
			if err := gdb.Exec(fmt.Sprintf("DELETE FROM %s", quoteIdent(t))).Error; err != nil {
				if datagorm.IsForeignKeyViolation(err) {
					deferred = append(deferred, t)
					lastErr = err
					continue
				}
				return fmt.Errorf("table reset: delete from %s: %w", t, err)
			}
			deleted++
		}
		if len(deferred) == len(remaining) {
			return fmt.Errorf("table reset: no progress on pass %d, %d tables still referenced: %v: %w",
				pass, len(deferred), deferred, lastErr)
		}
		remaining = deferred
	}

	log.Info().Str("component", "gorm").Str("dialect", r.config.Dialect).Str("database", r.config.Name).
		Int("tables", deleted).Str("duration", time.Since(start).String()).Msg("table reset complete")
	return nil
}

func (r *tableReset) targets(db *gorm.DB) ([]string, error) {
	var tables []string
	if r.tables != nil {
		tables = r.tables
	} else {
		all, err := db.Migrator().GetTables()
		if err != nil {
			return nil, fmt.Errorf("table reset: list tables: %w", err)
		}
		tables = all
	}
	out := make([]string, 0, len(tables))
	for _, t := range tables {
		if strings.HasPrefix(t, "goose_migration_") || r.keep[t] {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// quoteIdent double-quotes an identifier. Table names come from the schema
// (introspection) or from the caller's own literal list, so this is belt
// and braces rather than injection defence.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
