package gorm

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
)

// MigratorData is one goose migration set: a filesystem, the directory inside
// it, and a name that becomes the set's own version table
// (goose_migration_<name>) so sets version independently.
type MigratorData struct {
	Name string
	FS   fs.FS
	Path string
}

type migrator struct {
	config *datagorm.Config
	data   []MigratorData
}

var dialectMap = map[string]goose.Dialect{
	"postgres":   goose.DialectPostgres,
	"pgx":        goose.DialectPostgres,
	"mysql":      goose.DialectMySQL,
	"sqlite3":    goose.DialectSQLite3,
	"sqlite":     goose.DialectSQLite3,
	"mssql":      goose.DialectMSSQL,
	"azuresql":   goose.DialectMSSQL,
	"sqlserver":  goose.DialectMSSQL,
	"redshift":   goose.DialectRedshift,
	"tidb":       goose.DialectTiDB,
	"clickhouse": goose.DialectClickHouse,
	"ydb":        goose.DialectYdB,
	"turso":      goose.DialectTurso,
	"starrocks":  goose.DialectStarrocks,
}

func dialectFromString(s string) (goose.Dialect, error) {
	if d, ok := dialectMap[s]; ok {
		return d, nil
	}
	return "", fmt.Errorf("%q: unknown dialect", s)
}

// withDB opens a pool for the duration of fn and closes it afterwards. A
// migrator is invoked a handful of times per process; holding a pool open
// between invocations would leak a connection per migrator, which is exactly
// the kind of slow leak that exhausts a test database's connection slots.
func (m *migrator) withDB(fn func(db *gorm.DB) error) error {
	gdb, err := datagorm.NewDB(m.config)
	if err != nil {
		return err
	}
	defer func() {
		if sdb, derr := gdb.DB(); derr == nil {
			_ = sdb.Close()
		}
	}()
	return fn(gdb)
}

func (m *migrator) provider(db *gorm.DB, d MigratorData) (*goose.Provider, error) {
	sdb, err := db.DB()
	if err != nil {
		return nil, err
	}
	dialect, err := dialectFromString(m.config.Dialect)
	if err != nil {
		return nil, err
	}
	migFS, err := fs.Sub(d.FS, d.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-FS for path %q: %w", d.Path, err)
	}
	provider, err := goose.NewProvider(dialect, sdb, migFS,
		goose.WithTableName(fmt.Sprintf("goose_migration_%s", d.Name)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create goose provider for %q: %w", d.Name, err)
	}
	return provider, nil
}

// Migrate applies every set, in order. It is the Migrator contract and is
// identical to Up.
func (m *migrator) Migrate(ctx context.Context) error {
	return m.Up(ctx)
}

// Up applies every set, in order.
func (m *migrator) Up(ctx context.Context) error {
	return m.withDB(func(db *gorm.DB) error {
		for _, d := range m.data {
			provider, err := m.provider(db, d)
			if err != nil {
				return err
			}
			start := time.Now()
			if err := upWithRetry(ctx, provider, d.Name); err != nil {
				return err
			}
			m.logSet(d.Name, "up", start)
		}
		return nil
	})
}

// Down reverts every set to version zero, in reverse order of application.
// Sets without `-- +goose Down` sections are reverted as far as goose can
// take them: the version rows go, the effects stay. NewDeltaMigrator refuses
// such sets up front; NewMigrator (the base tier) never runs Down in the
// lifecycle and accepts them.
func (m *migrator) Down(ctx context.Context) error {
	return m.withDB(func(db *gorm.DB) error {
		for i := len(m.data) - 1; i >= 0; i-- {
			d := m.data[i]
			provider, err := m.provider(db, d)
			if err != nil {
				return err
			}
			start := time.Now()
			if _, err := provider.DownTo(ctx, 0); err != nil {
				return fmt.Errorf("migration %q down failed: %w", d.Name, err)
			}
			m.logSet(d.Name, "down", start)
		}
		return nil
	})
}

func (m *migrator) logSet(name, direction string, start time.Time) {
	log.Info().Str("component", "gorm").Str("dialect", m.config.Dialect).Str("database", m.config.Name).
		Str("name", name).Str("direction", direction).Str("duration", time.Since(start).String()).
		Msg("migration set complete")
}

func upWithRetry(ctx context.Context, provider *goose.Provider, name string) error {
	const (
		attempts = 5
		baseWait = 200 * time.Millisecond
	)
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if _, err = provider.Up(ctx); err == nil {
			return nil
		}
		if !datagorm.IsSerializationFailure(err) {
			break
		}
		log.Warn().Str("component", "gorm").Str("name", name).Int("attempt", attempt).Err(err).Msg("retrying migration set after serialization failure")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * baseWait):
		}
	}
	return fmt.Errorf("migration %q failed: %w", name, err)
}

// MigratorParams collects the base tier: the untagged []MigratorData.
type MigratorParams struct {
	fx.In
	Config *MigratorGormConfig
	Data   []MigratorData
}

// NewMigrator builds the base-tier Migrator from the untagged []MigratorData
// in the graph. It is applied once per database and never reverted by the
// lifecycle, so its sets need no Down sections.
func NewMigrator(params MigratorParams) migrate.Migrator {
	return &migrator{
		config: &params.Config.GormConfig,
		data:   params.Data,
	}
}

// DeltaParams collects the delta tier: the []MigratorData tagged name:"delta".
type DeltaParams struct {
	fx.In
	Config *MigratorGormConfig
	Data   []MigratorData `name:"delta"`
}

// DeltaResult is the delta-tier Reversible, tagged name:"delta" so the test
// lifecycle can find it.
type DeltaResult struct {
	fx.Out
	Delta migrate.Reversible `name:"delta"`
}

var gooseDownMarker = regexp.MustCompile(`(?m)^\s*--\s*\+goose\s+Down\b`)

// NewDeltaMigrator builds the delta-tier Reversible from goose sets. Every SQL
// file in every set must carry a `-- +goose Down` section: goose treats a
// missing section as an empty migration and silently leaves the effects in
// place, which in the delta tier would leak one test's rows into the next.
func NewDeltaMigrator(params DeltaParams) (DeltaResult, error) {
	for _, d := range params.Data {
		if err := requireDownSections(d); err != nil {
			return DeltaResult{}, err
		}
	}
	return DeltaResult{Delta: &migrator{
		config: &params.Config.GormConfig,
		data:   params.Data,
	}}, nil
}

func requireDownSections(d MigratorData) error {
	sub, err := fs.Sub(d.FS, d.Path)
	if err != nil {
		return fmt.Errorf("delta set %q: %w", d.Name, err)
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return fmt.Errorf("delta set %q: %w", d.Name, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := fs.ReadFile(sub, e.Name())
		if err != nil {
			return fmt.Errorf("delta set %q: %w", d.Name, err)
		}
		if !gooseDownMarker.Match(b) {
			return fmt.Errorf("delta set %q: %s has no `-- +goose Down` section; delta migrations must be reversible",
				d.Name, path.Join(d.Path, e.Name()))
		}
	}
	return nil
}
