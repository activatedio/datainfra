package gorm_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	"github.com/activatedio/datainfra/pkg/setup/gorm"
)

func TestSetup_Success(t *testing.T) {

	r := require.New(t)

	type s struct {
		arrange func() gorm.SetupParams
	}

	ctx := context.Background()

	cases := map[string]s{
		"default": {
			arrange: func() gorm.SetupParams {

				now := time.Now().UnixMilli()
				name := fmt.Sprintf("test_%d", now)

				return gorm.SetupParams{
					OwnerConfig: &gorm.OwnerGormConfig{
						Config: datagorm.Config{
							Dialect:  "postgres",
							Host:     "127.0.0.1",
							Port:     5432,
							Username: "postgres",
							Password: "supersecret",
							Name:     "postgres",
						},
					},
					AppConfig: &datagorm.Config{
						Dialect:  "postgres",
						Host:     "127.0.0.1",
						Port:     5432,
						Username: name,
						Password: name,
						Name:     name,
					},
				}
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(_ *testing.T) {

			unit := gorm.NewSetup(v.arrange())

			err := unit.Teardown(ctx)

			r.NoError(err)

			err = unit.Setup(ctx, setup.Params{FailOnExisting: true})

			r.NoError(err)

			err = unit.Setup(ctx, setup.Params{FailOnExisting: false})

			r.NoError(err)

			err = unit.Setup(ctx, setup.Params{FailOnExisting: true})

			r.ErrorAs(err, &setup.ResourceExistsError{})

			err = unit.Teardown(ctx)

			r.NoError(err)

			err = unit.Setup(ctx, setup.Params{FailOnExisting: true})

			r.NoError(err)

			err = unit.Teardown(ctx)

			r.NoError(err)

		})
	}

}

func TestTeardown_Sqlite(t *testing.T) {

	ctx := context.Background()

	// newSetup builds a sqlite Setup pointed at path. Both configs carry the
	// dialect because Teardown dispatches on the owner's and teardownSqlite
	// reads the app's name.
	newSetup := func(path string) setup.Setup {
		return gorm.NewSetup(gorm.SetupParams{
			OwnerConfig: &gorm.OwnerGormConfig{
				Config: datagorm.Config{Dialect: "sqlite", Name: path},
			},
			AppConfig: &datagorm.Config{Dialect: "sqlite", Name: path},
		})
	}

	type s struct {
		arrange func(t *testing.T, dir string) string
		assert  func(t *testing.T, path string, err error)
	}

	cases := map[string]s{
		// The regression: this used to be a no-op and the file survived.
		"removes the database file": {
			arrange: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "test.db")
				require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))
				return path
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				require.NoFileExists(t, path)
			},
		},
		// WAL mode leaves these beside the database; they are part of it.
		"removes the wal and shm sidecars": {
			arrange: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "test.db")
				for _, p := range []string{path, path + "-wal", path + "-shm"} {
					require.NoError(t, os.WriteFile(p, []byte("data"), 0o600))
				}
				return path
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				require.NoFileExists(t, path)
				require.NoFileExists(t, path+"-wal")
				require.NoFileExists(t, path+"-shm")
			},
		},
		// Teardown is routinely called before Setup to clear a dirty slate
		// (TestSetup_Success does exactly that), so absence must succeed.
		"a missing file is not an error": {
			arrange: func(_ *testing.T, dir string) string {
				return filepath.Join(dir, "never-existed.db")
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				require.NoFileExists(t, path)
			},
		},
		"an in-memory database is a no-op": {
			arrange: func(_ *testing.T, _ string) string {
				return ":memory:"
			},
			assert: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
			},
		},
		"a file: uri in-memory database is a no-op": {
			arrange: func(_ *testing.T, _ string) string {
				return "file::memory:?cache=shared"
			},
			assert: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
			},
		},
		// A DSN's query parameters are not part of the filename.
		"strips a file: prefix and query parameters": {
			arrange: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "test.db")
				require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))
				return "file:" + path + "?_pragma=busy_timeout(1000)"
			},
			assert: func(t *testing.T, dsn string, err error) {
				require.NoError(t, err)
				path := strings.TrimPrefix(strings.Split(dsn, "?")[0], "file:")
				require.NoFileExists(t, path)
			},
		},
		"an empty name is a no-op": {
			arrange: func(_ *testing.T, _ string) string {
				return ""
			},
			assert: func(t *testing.T, _ string, err error) {
				require.NoError(t, err)
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			dir := t.TempDir()
			name := v.arrange(t, dir)
			v.assert(t, name, newSetup(name).Teardown(ctx))
		})
	}
}

// ownerFromEnv is the owner the Postgres tests connect as:
// DATAINFRA_PG_{HOST,PORT,USER,PASSWORD}, defaulting to the values
// TestSetup_Success uses.
func ownerFromEnv() datagorm.Config {
	env := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	port := 5432
	_, _ = fmt.Sscanf(env("DATAINFRA_PG_PORT", "5432"), "%d", &port)
	return datagorm.Config{Dialect: "postgres", Host: env("DATAINFRA_PG_HOST", "127.0.0.1"), Port: port,
		Username: env("DATAINFRA_PG_USER", "postgres"), Password: env("DATAINFRA_PG_PASSWORD", "supersecret"), Name: "postgres"}
}

// Two apps in one database, each in its own schema: the second finds the
// database there and still gets its schema, and neither sees the other's
// tables.
func TestSetup_SchemaPerApp(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	owner := ownerFromEnv()
	if db, err := datagorm.NewDB(&owner); err != nil {
		t.Skipf("no postgres: %v", err)
	} else if sdb, _ := db.DB(); sdb.Ping() != nil {
		t.Skip("no postgres")
	}

	name := fmt.Sprintf("schema_%d", time.Now().UnixMilli())
	app := func(schema string) *datagorm.Config {
		return &datagorm.Config{Dialect: "postgres", Host: owner.Host, Port: owner.Port,
			Username: owner.Username, Password: owner.Password, Name: name, Schema: schema}
	}
	fleet := gorm.NewSetup(gorm.SetupParams{OwnerConfig: &gorm.OwnerGormConfig{Config: owner}, AppConfig: app("fleet")})
	signs := gorm.NewSetup(gorm.SetupParams{OwnerConfig: &gorm.OwnerGormConfig{Config: owner}, AppConfig: app("signs")})
	defer func() { r.NoError(fleet.Teardown(ctx)) }()

	r.NoError(fleet.Setup(ctx, setup.Params{}))
	r.NoError(signs.Setup(ctx, setup.Params{}), "the second app finds the database and still gets its schema")
	r.NoError(fleet.Setup(ctx, setup.Params{}), "idempotent")

	fdb, err := datagorm.NewDB(app("fleet"))
	r.NoError(err)
	r.NoError(fdb.Exec("CREATE TABLE devices (id text)").Error)
	var where string
	r.NoError(fdb.Raw("SELECT table_schema FROM information_schema.tables WHERE table_name = 'devices'").Scan(&where).Error)
	r.Equal("fleet", where)

	sdb, err := datagorm.NewDB(app("signs"))
	r.NoError(err)
	r.Error(sdb.Exec("SELECT * FROM devices").Error, "another schema's table is invisible")
	r.NoError(sdb.Exec("CREATE TABLE devices (id text)").Error, "and its name is free")

	if s, err := fdb.DB(); err == nil {
		_ = s.Close()
	}
	if s, err := sdb.DB(); err == nil {
		_ = s.Close()
	}
}
