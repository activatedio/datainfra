package gorm_test

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
)

func openSqlite(t *testing.T) (*gorm.DB, *gormmigrate.MigratorGormConfig) {
	cfg := datagorm.Config{Dialect: datagorm.DialectSqlite, Name: filepath.Join(t.TempDir(), "layer.db")}
	db, err := datagorm.NewDB(&cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sdb, _ := db.DB()
		_ = sdb.Close()
	})
	return db, &gormmigrate.MigratorGormConfig{GormConfig: cfg}
}

const withDown = "-- +goose Up\nCREATE TABLE a (id int PRIMARY KEY);\n-- +goose Down\nDROP TABLE a;\n"

// b references a, so applying must go a then b and reverting b then a.
const withDownB = "-- +goose Up\nCREATE TABLE b (id int REFERENCES a(id));\n-- +goose Down\nDROP TABLE b;\n"

func TestNewGooseLayerRequiresDownSections(t *testing.T) {

	cases := map[string]struct {
		arrange func() fstest.MapFS
		assert  func(t *testing.T, err error)
	}{
		"every file has a down section": {
			arrange: func() fstest.MapFS {
				return fstest.MapFS{
					"m/001_a.sql": {Data: []byte(withDown)},
					"m/002_b.sql": {Data: []byte("-- +goose Up\nCREATE TABLE b (id int);\n  -- +goose Down\nDROP TABLE b;\n")},
				}
			},
			assert: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		"a file without a down section is refused by name": {
			arrange: func() fstest.MapFS {
				return fstest.MapFS{
					"m/001_a.sql": {Data: []byte(withDown)},
					"m/002_b.sql": {Data: []byte("-- +goose Up\nCREATE TABLE b (id int);\n")},
				}
			},
			assert: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "m/002_b.sql")
				require.ErrorContains(t, err, "+goose Down")
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			_, cfg := openSqlite(t)
			_, err := gormmigrate.NewGooseLayer(cfg, gormmigrate.MigratorData{Name: "test", FS: v.arrange(), Path: "m"})
			v.assert(t, err)
		})
	}
}

func TestGooseLayer(t *testing.T) {

	set := fstest.MapFS{"m/001_a.sql": {Data: []byte(withDown)}, "m/002_b.sql": {Data: []byte(withDownB)}}
	data := gormmigrate.MigratorData{Name: "test", FS: set, Path: "m"}

	cases := map[string]struct {
		arrange func(cfg *gormmigrate.MigratorGormConfig) migrate.Layer
		assert  func(t *testing.T, db *gorm.DB, l migrate.Layer)
	}{
		"up applies in order, down reverts in reverse, and no version table is kept": {
			arrange: func(cfg *gormmigrate.MigratorGormConfig) migrate.Layer {
				l, err := gormmigrate.NewGooseLayer(cfg, data)
				require.NoError(t, err)
				return l
			},
			assert: func(t *testing.T, db *gorm.DB, l migrate.Layer) {
				require.Equal(t, "test", l.Name())
				_, resettable := l.(migrate.Resettable)
				require.False(t, resettable, "no reset unless given one")

				require.NoError(t, l.Up(context.Background()))
				require.True(t, db.Migrator().HasTable("a"))
				require.True(t, db.Migrator().HasTable("b"))
				require.False(t, db.Migrator().HasTable("goose_migration_test"),
					"the planner records what a store carries; a goose version table would survive a Reset and lie")

				// A referencing row makes the order observable: dropping a
				// before b would fail the foreign key.
				require.NoError(t, db.Exec(`INSERT INTO a VALUES (1)`).Error)
				require.NoError(t, db.Exec(`INSERT INTO b VALUES (1)`).Error)
				require.NoError(t, l.Down(context.Background()))
				require.False(t, db.Migrator().HasTable("b"))
				require.False(t, db.Migrator().HasTable("a"))

				// Up again after Down: nothing remembers the first run.
				require.NoError(t, l.Up(context.Background()))
				require.True(t, db.Migrator().HasTable("b"))
			},
		},
		"with reset the layer is resettable and runs the given function": {
			arrange: func(cfg *gormmigrate.MigratorGormConfig) migrate.Layer {
				l, err := gormmigrate.NewGooseLayer(cfg, data, gormmigrate.WithReset(func(_ context.Context, db *gorm.DB) error {
					return db.Exec(`DELETE FROM a`).Error
				}))
				require.NoError(t, err)
				return l
			},
			assert: func(t *testing.T, db *gorm.DB, l migrate.Layer) {
				require.NoError(t, l.Up(context.Background()))
				require.NoError(t, db.Exec(`INSERT INTO a VALUES (1)`).Error)
				r, ok := l.(migrate.Resettable)
				require.True(t, ok)
				require.NoError(t, r.Reset(context.Background()))
				var n int64
				require.NoError(t, db.Raw(`SELECT count(*) FROM a`).Scan(&n).Error)
				require.EqualValues(t, 0, n)
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			db, cfg := openSqlite(t)
			v.assert(t, db, v.arrange(cfg))
		})
	}
}
