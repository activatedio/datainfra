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
