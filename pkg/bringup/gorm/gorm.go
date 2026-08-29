package gorm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/activatedio/datainfra/pkg/bringup"
	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/migrate"
	"github.com/activatedio/datainfra/pkg/setup"
)

// DefaultTimeout bounds setup and migrations together. Bring-up is quick
// when healthy; the bound exists so a wedged database fails the boot rather
// than hanging it.
const DefaultTimeout = 5 * time.Minute

// Options selects which bring-up stages run before the pool opens. The zero
// value is the pre-provisioned posture: no stages, pool gated on a no-op
// Ready.
type Options struct {
	// Setup creates the database and role under owner credentials before
	// migrating. Requires a setup.Setup in the graph; requesting it without
	// one is a construction error rather than a silent skip.
	Setup bool
	// Migrate applies the registered migrations at bring-up. Requires a
	// migrate.Migrator in the graph, on the same terms.
	Migrate bool
	// Timeout bounds setup and migrations together. Zero means
	// DefaultTimeout.
	Timeout time.Duration
}

// readyParams collects the stage implementations. Both are optional in the
// graph: which of them must exist is Options' decision, checked in
// newReady — an absent stage that was requested fails loudly, while a
// present stage that was not requested is ignored (a migrate command's graph
// can share providers with a server graph without the server re-migrating).
type readyParams struct {
	fx.In

	Setup    setup.Setup      `optional:"true"`
	Migrator migrate.Migrator `optional:"true"`
}

// Module wires the ordered bring-up: *bringup.Ready (running the selected
// stages), the ready-gated *gorm.DB, and the unwrapped *sql.DB for consumers
// on database/sql. Configuration (*datagorm.Config, and the stage
// implementations when requested) comes from the enclosing graph.
func Module(options Options) fx.Option {
	return fx.Module("datainfra.bringup.gorm",
		fx.Provide(
			func(params readyParams) (*bringup.Ready, error) {
				return newReady(options, params)
			},
			NewDB,
			NewSQLDB,
		),
	)
}

// newReady runs the selected stages and returns the marker the pool depends
// on.
func newReady(options Options, params readyParams) (*bringup.Ready, error) {

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if options.Setup {
		if params.Setup == nil {
			return nil, errors.New("bringup: options request setup but no setup.Setup is in the graph")
		}
		log.Info().Msg("bringup: running setup")
		if err := params.Setup.Setup(ctx, setup.Params{}); err != nil {
			return nil, fmt.Errorf("bringup: setup: %w", err)
		}
	}

	if options.Migrate {
		if params.Migrator == nil {
			return nil, errors.New("bringup: options request migrations but no migrate.Migrator is in the graph")
		}
		log.Info().Msg("bringup: running migrations")
		if err := params.Migrator.Migrate(ctx); err != nil {
			return nil, fmt.Errorf("bringup: migrate: %w", err)
		}
	}

	return &bringup.Ready{}, nil
}

// NewDB opens the pooled application connection once bring-up has finished.
// It stands in for datagorm.NewDB purely to carry the ordering dependency.
func NewDB(config *datagorm.Config, _ *bringup.Ready) (*gorm.DB, error) {
	return datagorm.NewDB(config)
}

// NewSQLDB unwraps the pool for consumers that speak database/sql rather
// than gorm. It is the same pool, not a second one, so bring-up ordering and
// connection limits hold for both.
func NewSQLDB(db *gorm.DB) (*sql.DB, error) {
	return db.DB()
}
