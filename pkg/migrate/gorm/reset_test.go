package gorm_test

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	gormmigrate "github.com/activatedio/datainfra/pkg/migrate/gorm"
)

func openSqlite(t *testing.T) (*gorm.DB, *gormmigrate.MigratorGormConfig) {
	cfg := datagorm.Config{Dialect: datagorm.DialectSqlite, Name: filepath.Join(t.TempDir(), "reset.db")}
	db, err := datagorm.NewDB(&cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sdb, _ := db.DB()
		_ = sdb.Close()
	})
	return db, &gormmigrate.MigratorGormConfig{GormConfig: cfg}
}

func count(t *testing.T, db *gorm.DB, table string) int64 {
	var n int64
	require.NoError(t, db.Raw("SELECT count(*) FROM "+table).Scan(&n).Error)
	return n
}

// The schema is a three-deep foreign-key chain declared parent-first, so a
// naive parent-first DELETE trips the constraint and the reset has to defer.
const fkSchema = `
CREATE TABLE parents (id INTEGER PRIMARY KEY);
CREATE TABLE children (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parents(id));
CREATE TABLE grandchildren (id INTEGER PRIMARY KEY, child_id INTEGER NOT NULL REFERENCES children(id));
CREATE TABLE goose_migration_main (id INTEGER PRIMARY KEY);
INSERT INTO parents VALUES (1);
INSERT INTO children VALUES (1, 1);
INSERT INTO grandchildren VALUES (1, 1);
INSERT INTO goose_migration_main VALUES (7);
`

func TestTableReset(t *testing.T) {

	cases := map[string]struct {
		arrange func(cfg *gormmigrate.MigratorGormConfig) []gormmigrate.TableResetOption
		assert  func(t *testing.T, db *gorm.DB, err error)
	}{
		"introspected tables clear in dependency order and keep goose": {
			arrange: func(_ *gormmigrate.MigratorGormConfig) []gormmigrate.TableResetOption { return nil },
			assert: func(t *testing.T, db *gorm.DB, err error) {
				require.NoError(t, err)
				require.EqualValues(t, 0, count(t, db, "parents"))
				require.EqualValues(t, 0, count(t, db, "children"))
				require.EqualValues(t, 0, count(t, db, "grandchildren"))
				require.EqualValues(t, 1, count(t, db, "goose_migration_main"))
			},
		},
		"explicit table list is honoured": {
			arrange: func(_ *gormmigrate.MigratorGormConfig) []gormmigrate.TableResetOption {
				return []gormmigrate.TableResetOption{gormmigrate.WithTables("grandchildren")}
			},
			assert: func(t *testing.T, db *gorm.DB, err error) {
				require.NoError(t, err)
				require.EqualValues(t, 0, count(t, db, "grandchildren"))
				require.EqualValues(t, 1, count(t, db, "children"))
			},
		},
		"kept tables are left alone": {
			arrange: func(_ *gormmigrate.MigratorGormConfig) []gormmigrate.TableResetOption {
				return []gormmigrate.TableResetOption{gormmigrate.WithKeep("parents", "children")}
			},
			assert: func(t *testing.T, db *gorm.DB, err error) {
				require.NoError(t, err)
				require.EqualValues(t, 0, count(t, db, "grandchildren"))
				require.EqualValues(t, 1, count(t, db, "children"))
				require.EqualValues(t, 1, count(t, db, "parents"))
			},
		},
		"a kept child blocks its parent and the reset says so": {
			arrange: func(_ *gormmigrate.MigratorGormConfig) []gormmigrate.TableResetOption {
				return []gormmigrate.TableResetOption{gormmigrate.WithKeep("grandchildren")}
			},
			assert: func(t *testing.T, _ *gorm.DB, err error) {
				require.ErrorContains(t, err, "no progress")
				require.ErrorContains(t, err, "children")
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			db, cfg := openSqlite(t)
			require.NoError(t, db.Exec(fkSchema).Error)
			reset := gormmigrate.NewTableReset(cfg, v.arrange(cfg)...)
			require.NoError(t, reset.Up(context.Background()))
			err := reset.Down(context.Background())
			v.assert(t, db, err)
		})
	}
}

func TestNewDeltaMigratorRequiresDownSections(t *testing.T) {

	cases := map[string]struct {
		arrange func() fstest.MapFS
		assert  func(t *testing.T, err error)
	}{
		"every file has a down section": {
			arrange: func() fstest.MapFS {
				return fstest.MapFS{
					"m/001_a.sql": {Data: []byte("-- +goose Up\nCREATE TABLE a (id int);\n-- +goose Down\nDROP TABLE a;\n")},
					"m/002_b.sql": {Data: []byte("-- +goose Up\nCREATE TABLE b (id int);\n  -- +goose Down\nDROP TABLE b;\n")},
				}
			},
			assert: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		"a file without a down section is refused by name": {
			arrange: func() fstest.MapFS {
				return fstest.MapFS{
					"m/001_a.sql": {Data: []byte("-- +goose Up\nCREATE TABLE a (id int);\n-- +goose Down\nDROP TABLE a;\n")},
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
			_, err := gormmigrate.NewDeltaMigrator(gormmigrate.DeltaParams{
				Config: cfg,
				Data:   []gormmigrate.MigratorData{{Name: "test", FS: v.arrange(), Path: "m"}},
			})
			v.assert(t, err)
		})
	}
}

// Up then Down through goose on a real set: the base tier's migrator applies
// and the same set reverts to version zero.
func TestMigratorUpDown(t *testing.T) {
	db, cfg := openSqlite(t)
	set := fstest.MapFS{
		"m/001_a.sql": {Data: []byte("-- +goose Up\nCREATE TABLE a (id int);\n-- +goose Down\nDROP TABLE a;\n")},
	}
	res, err := gormmigrate.NewDeltaMigrator(gormmigrate.DeltaParams{
		Config: cfg,
		Data:   []gormmigrate.MigratorData{{Name: "test", FS: set, Path: "m"}},
	})
	require.NoError(t, err)

	require.NoError(t, res.Delta.Up(context.Background()))
	require.True(t, db.Migrator().HasTable("a"))

	require.NoError(t, res.Delta.Down(context.Background()))
	require.False(t, db.Migrator().HasTable("a"))
	var maxVersion int64
	require.NoError(t, db.Raw(`SELECT max(version_id) FROM goose_migration_test`).Scan(&maxVersion).Error)
	require.EqualValues(t, 0, maxVersion, "version rows reverted to the goose baseline")
}
