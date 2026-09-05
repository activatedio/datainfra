package testing_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	gormtesting "github.com/activatedio/datainfra/pkg/data/gorm/testing"
	datatesting "github.com/activatedio/datainfra/pkg/data/testing"
	"github.com/activatedio/datainfra/pkg/migrate"
	gormsetup "github.com/activatedio/datainfra/pkg/setup/gorm"
)

// The planner is exercised on sqlite so it runs anywhere. The rings are
// dialect-independent: Setup/Teardown dispatch inside setup/gorm, which has
// its own per-dialect tests.

// counts records how many times each direction ran on a layer.
type counts struct {
	mu                    sync.Mutex
	up, down, reset, keyN int
}

func (c *counts) inc(p *int) {
	c.mu.Lock()
	*p++
	c.mu.Unlock()
}

// schema owns the marker table. Its Reset clears the table: the state a
// fresh Up would leave, with nothing above it applied.
type schema struct {
	db        func() *gorm.DB
	c         counts
	resetable bool
	failDown  bool
}

func (s *schema) Name() string { return "schema" }
func (s *schema) Up(_ context.Context) error {
	s.c.inc(&s.c.up)
	return s.db().Exec(`CREATE TABLE marker (id INTEGER PRIMARY KEY, src TEXT)`).Error
}
func (s *schema) Down(_ context.Context) error {
	s.c.inc(&s.c.down)
	if s.failDown {
		return errors.New("schema down refused")
	}
	return s.db().Exec(`DROP TABLE marker`).Error
}

type resettableSchema struct{ *schema }

func (s *resettableSchema) Reset(_ context.Context) error {
	s.c.inc(&s.c.reset)
	return s.db().Exec(`DELETE FROM marker`).Error
}

// seed inserts one row keyed by its parameter, and removes exactly that row.
type seed struct {
	db  func() *gorm.DB
	key string
	c   *counts
}

func (s *seed) Name() string { return "seed" }
func (s *seed) Key() string  { return s.key }
func (s *seed) Up(_ context.Context) error {
	s.c.inc(&s.c.up)
	return s.db().Exec(`INSERT INTO marker (id, src) VALUES (?, ?)`, 1, "seed:"+s.key).Error
}
func (s *seed) Down(_ context.Context) error {
	s.c.inc(&s.c.down)
	return s.db().Exec(`DELETE FROM marker WHERE id = 1 AND src = ?`, "seed:"+s.key).Error
}

type world struct {
	path   string
	open   func() *gorm.DB
	schema *schema
	seedC  counts
	fix    datatesting.LifecycleFixture
	// seedKey is read by the stack provider each time an app is built, so a
	// test can change the seed parameter between runs.
	seedKey string
}

func newWorld(t *testing.T, resettable bool) *world {
	w := &world{path: filepath.Join(t.TempDir(), "store.db"), seedKey: "a"}
	cfg := &datagorm.Config{Dialect: datagorm.DialectSqlite, Name: w.path}
	w.open = func() *gorm.DB {
		db, err := datagorm.NewDB(cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			sdb, _ := db.DB()
			_ = sdb.Close()
		})
		return db
	}
	w.schema = &schema{db: w.open, resetable: resettable}
	var schemaLayer migrate.Layer = w.schema
	if resettable {
		schemaLayer = &resettableSchema{w.schema}
	}
	w.fix = gormtesting.NewAppFixture("sqlite", fx.Module("sqlite-fixture",
		fx.Provide(
			func() *datagorm.Config { return cfg },
			func() *gormsetup.OwnerGormConfig { return &gormsetup.OwnerGormConfig{Config: *cfg} },
			datagorm.NewContextBuilder,
			func() []migrate.Layer {
				return []migrate.Layer{schemaLayer, &seed{db: w.open, key: w.seedKey, c: &w.seedC}}
			},
		),
	))
	return w
}

// run builds one test app under req, calls during while the app is up, and
// returns once the child test — and so the per-test rings — has finished.
func (w *world) run(t *testing.T, req datatesting.Requirement, during func(t *testing.T)) {
	t.Run("run", func(t *testing.T) {
		app := fxtest.New(t, fx.NopLogger, w.fix.GetApp(t, req).App)
		app.RequireStart()
		if during != nil {
			during(t)
		}
		app.RequireStop()
	})
}

func (w *world) rows(t *testing.T) []string {
	var out []string
	require.NoError(t, w.open().Raw(`SELECT src FROM marker ORDER BY id`).Scan(&out).Error)
	return out
}

func (w *world) dirty(t *testing.T) {
	require.NoError(t, w.open().Exec(`INSERT INTO marker (id, src) VALUES (99, 'test')`).Error)
}

func TestPlanner(t *testing.T) {

	full := datatesting.Requirement{}
	pristine := datatesting.Requirement{Tolerance: datatesting.Pristine}

	cases := map[string]struct {
		arrange func(t *testing.T) *world
		assert  func(t *testing.T, w *world)
	}{
		"tolerant tests share the store and keep each other's rows": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				w.run(t, full, func(t *testing.T) { w.dirty(t) })
				w.run(t, full, nil)
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, []string{"seed:a", "test"}, w.rows(t))
				require.Equal(t, 1, w.schema.c.up, "schema applied once")
				require.Equal(t, 1, w.seedC.up, "seed applied once")
				require.Equal(t, 0, w.schema.c.reset)
			},
		},
		"pristine after a dirty store resets the bottom layer and re-applies above": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				w.run(t, full, func(t *testing.T) { w.dirty(t) })
				w.run(t, pristine, func(t *testing.T) {
					require.Equal(t, []string{"seed:a"}, w.rows(t), "pristine while the test runs")
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 1, w.schema.c.reset)
				require.Equal(t, 0, w.schema.c.down, "reset spared the DDL")
				require.Equal(t, 2, w.seedC.up, "seed re-applied after the reset")
				require.Equal(t, 0, w.seedC.down, "reset discarded it; no exact down needed")
			},
		},
		"pristine on a clean store does nothing": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				w.run(t, pristine, nil)
				w.run(t, pristine, nil)
				return w
			},
			assert: func(t *testing.T, w *world) {
				// The first test is assumed to have written, so the second
				// still resets — the fixture cannot know it did not.
				require.Equal(t, 1, w.schema.c.reset)
				require.Equal(t, 1, w.schema.c.up)
			},
		},
		"pristine without a resettable bottom falls back to down and up": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, false)
				w.run(t, full, func(t *testing.T) { w.dirty(t) })
				w.run(t, pristine, func(t *testing.T) {
					require.Equal(t, []string{"seed:a"}, w.rows(t))
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 1, w.seedC.down, "seed reversed exactly, top first")
				require.Equal(t, 1, w.schema.c.down)
				require.Equal(t, 2, w.schema.c.up)
				require.Equal(t, 2, w.seedC.up)
			},
		},
		"a subset stack downs the layers above it": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				w.run(t, full, nil)
				w.run(t, datatesting.Requirement{Stack: []string{"schema"}}, func(t *testing.T) {
					require.Empty(t, w.rows(t))
				})
				w.run(t, full, nil)
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 1, w.seedC.down)
				require.Equal(t, 2, w.seedC.up)
				require.Equal(t, 1, w.schema.c.up, "schema untouched throughout")
			},
		},
		"a changed key reverses the old instance and applies the new one": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				w.run(t, full, nil)
				w.seedKey = "b"
				w.run(t, full, func(t *testing.T) {
					require.Equal(t, []string{"seed:b"}, w.rows(t))
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 1, w.seedC.down)
				require.Equal(t, 2, w.seedC.up)
			},
		},
		"a test that comes back for a store it holds is re-planned under its own hold": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				// One child test, two sequential apps: a loop over profiles or
				// two datatesting.Run calls. The second must not queue behind
				// the first's exclusive hold.
				t.Run("run", func(t *testing.T) {
					for i := 0; i < 2; i++ {
						app := fxtest.New(t, fx.NopLogger, w.fix.GetApp(t, pristine).App)
						app.RequireStart()
						w.dirty(t)
						app.RequireStop()
					}
					require.Equal(t, []string{"seed:a"}, w.rows(t)[:1])
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 1, w.schema.c.reset, "second ask reset the store the first phase dirtied")
				require.Equal(t, 2, w.seedC.up)
			},
		},
		"a shared holder asking for something else is refused, not deadlocked": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				t.Run("run", func(t *testing.T) {
					app := fxtest.New(t, fx.NopLogger, w.fix.GetApp(t, full).App)
					app.RequireStart()
					app.RequireStop()
					second := fx.New(fx.NopLogger, w.fix.GetApp(t, pristine).App)
					require.ErrorContains(t, second.Err(), "holds the store shared")
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 0, w.schema.c.reset)
			},
		},
		"a dedicated store is dropped when its test ends": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, true)
				w.run(t, datatesting.Requirement{Dedicated: true, Tolerance: datatesting.Pristine}, func(t *testing.T) {
					_, err := os.Stat(w.path)
					require.NoError(t, err)
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				_, err := os.Stat(w.path)
				require.ErrorIs(t, err, os.ErrNotExist)
				require.NoError(t, w.fix.Cleanup(), "cleanup on a dropped store is a no-op")
			},
		},
		"a failed step breaks the store and the next test gets it recreated": {
			arrange: func(t *testing.T) *world {
				w := newWorld(t, false)
				w.run(t, full, func(t *testing.T) { w.dirty(t) })
				w.schema.failDown = true
				// Pristine on a non-resettable dirty store: down seed, down
				// schema — which refuses. The app fails to start.
				// Invokes run at construction, so the failure surfaces from
				// fx.New itself; fxtest.New would fail the test for us.
				t.Run("broken", func(t *testing.T) {
					app := fx.New(fx.NopLogger, w.fix.GetApp(t, pristine).App)
					require.ErrorContains(t, app.Err(), "schema down refused")
				})
				w.schema.failDown = false
				w.run(t, full, func(t *testing.T) {
					require.Equal(t, []string{"seed:a"}, w.rows(t), "recreated store carries the full stack, clean")
				})
				return w
			},
			assert: func(t *testing.T, w *world) {
				require.Equal(t, 2, w.schema.c.up, "schema applied on the recreated store")
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			w := v.arrange(t)
			v.assert(t, w)
		})
	}
}

func TestRequirementResolution(t *testing.T) {

	cases := map[string]struct {
		arrange func() datatesting.Requirement
		assert  func(t *testing.T, err error)
	}{
		"unknown layer name": {
			arrange: func() datatesting.Requirement { return datatesting.Requirement{Stack: []string{"nope"}} },
			assert: func(t *testing.T, err error) {
				require.ErrorContains(t, err, `layer "nope"`)
				require.ErrorContains(t, err, "does not declare")
			},
		},
		"layers out of stack order": {
			arrange: func() datatesting.Requirement { return datatesting.Requirement{Stack: []string{"seed", "schema"}} },
			assert: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "out of stack order")
			},
		},
	}

	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			w := newWorld(t, true)
			app := fx.New(fx.NopLogger, w.fix.GetApp(t, v.arrange()).App)
			v.assert(t, app.Err())
		})
	}
}
