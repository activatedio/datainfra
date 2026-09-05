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
// migration runs a handful of times per process; holding a pool open between
// runs would leak a connection per migrator, which is exactly the kind of
// slow leak that exhausts a test database's connection slots.
func withDB(config *datagorm.Config, fn func(db *gorm.DB) error) error {
	gdb, err := datagorm.NewDB(config)
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

func newProvider(config *datagorm.Config, db *gorm.DB, d MigratorData) (*goose.Provider, error) {
	sdb, err := db.DB()
	if err != nil {
		return nil, err
	}
	dialect, err := dialectFromString(config.Dialect)
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

func logSet(config *datagorm.Config, name, direction string, start time.Time) {
	log.Info().Str("component", "gorm").Str("dialect", config.Dialect).Str("database", config.Name).
		Str("name", name).Str("direction", direction).Str("duration", time.Since(start).String()).
		Msg("migration set complete")
}

// ---- production migrator -------------------------------------------------

type migrator struct {
	config *datagorm.Config
	data   []MigratorData
}

// Migrate applies every set, in order.
func (m *migrator) Migrate(ctx context.Context) error {
	return withDB(m.config, func(db *gorm.DB) error {
		for _, d := range m.data {
			provider, err := newProvider(m.config, db, d)
			if err != nil {
				return err
			}
			start := time.Now()
			if err := upWithRetry(ctx, provider, d.Name); err != nil {
				return err
			}
			logSet(m.config, d.Name, "up", start)
		}
		return nil
	})
}

// MigratorParams collects the production migration sets.
type MigratorParams struct {
	fx.In
	Config *MigratorGormConfig
	Data   []MigratorData
}

// NewMigrator builds the production Migrator the bring-up module runs: every
// []MigratorData set in the graph, applied in order, never reverted.
func NewMigrator(params MigratorParams) migrate.Migrator {
	return &migrator{
		config: &params.Config.GormConfig,
		data:   params.Data,
	}
}

// ---- test layer ----------------------------------------------------------

// GooseLayerOption configures NewGooseLayer.
type GooseLayerOption func(*gooseLayer)

// WithReset gives the layer a Reset: fn runs against a pool on the layer's
// database and must return it to the state a fresh Up of this layer would
// produce, with nothing above it applied. A schema layer typically issues
// one TRUNCATE over the tables it created.
func WithReset(fn func(ctx context.Context, db *gorm.DB) error) GooseLayerOption {
	return func(l *gooseLayer) {
		l.reset = fn
	}
}

type gooseLayer struct {
	config *datagorm.Config
	data   MigratorData
	reset  func(ctx context.Context, db *gorm.DB) error
}

func (l *gooseLayer) Name() string { return l.data.Name }

func (l *gooseLayer) Up(ctx context.Context) error {
	return withDB(l.config, func(db *gorm.DB) error {
		provider, err := newProvider(l.config, db, l.data)
		if err != nil {
			return err
		}
		start := time.Now()
		if err := upWithRetry(ctx, provider, l.data.Name); err != nil {
			return err
		}
		logSet(l.config, l.data.Name, "up", start)
		return nil
	})
}

// Down reverts the set to version zero through its `-- +goose Down`
// sections, which NewGooseLayer verified are present in every file.
func (l *gooseLayer) Down(ctx context.Context) error {
	return withDB(l.config, func(db *gorm.DB) error {
		provider, err := newProvider(l.config, db, l.data)
		if err != nil {
			return err
		}
		start := time.Now()
		if _, err := provider.DownTo(ctx, 0); err != nil {
			return fmt.Errorf("migration %q down failed: %w", l.data.Name, err)
		}
		logSet(l.config, l.data.Name, "down", start)
		return nil
	})
}

// resettableGooseLayer is a gooseLayer with a Reset. It is a separate type so
// that only a layer given WithReset satisfies migrate.Resettable.
type resettableGooseLayer struct {
	*gooseLayer
}

func (l *resettableGooseLayer) Reset(ctx context.Context) error {
	return withDB(l.config, func(db *gorm.DB) error {
		start := time.Now()
		if err := l.reset(ctx, db.WithContext(ctx)); err != nil {
			return fmt.Errorf("migration %q reset failed: %w", l.data.Name, err)
		}
		logSet(l.config, l.data.Name, "reset", start)
		return nil
	})
}

var gooseDownMarker = regexp.MustCompile(`(?m)^\s*--\s*\+goose\s+Down\b`)

// NewGooseLayer builds a test Layer from one goose set. Every SQL file in the
// set must carry a `-- +goose Down` section: goose treats a missing section as
// an empty migration and silently leaves the effects in place, which would
// make Down a lie. A set that cannot be reversed cannot be a layer.
func NewGooseLayer(config *MigratorGormConfig, data MigratorData, opts ...GooseLayerOption) (migrate.Layer, error) {
	if err := requireDownSections(data); err != nil {
		return nil, err
	}
	l := &gooseLayer{config: &config.GormConfig, data: data}
	for _, o := range opts {
		o(l)
	}
	if l.reset != nil {
		return &resettableGooseLayer{l}, nil
	}
	return l, nil
}

func requireDownSections(d MigratorData) error {
	sub, err := fs.Sub(d.FS, d.Path)
	if err != nil {
		return fmt.Errorf("layer %q: %w", d.Name, err)
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return fmt.Errorf("layer %q: %w", d.Name, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := fs.ReadFile(sub, e.Name())
		if err != nil {
			return fmt.Errorf("layer %q: %w", d.Name, err)
		}
		if !gooseDownMarker.Match(b) {
			return fmt.Errorf("layer %q: %s has no `-- +goose Down` section; every layer must state its exact reverse",
				d.Name, path.Join(d.Path, e.Name()))
		}
	}
	return nil
}
