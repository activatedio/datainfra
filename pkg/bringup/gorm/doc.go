// Package gorm composes datainfra's setup, migrate and connection pieces
// into one ordered bring-up for fx graphs.
//
// Both bring-up postures are the same module with different options:
//
//   - Self-provisioning (Options{Setup: true, Migrate: true}): the process
//     creates its database and role under owner credentials and applies
//     migrations at boot, before the application pool dials.
//   - Pre-provisioned (Options{}): setup and migrations ran elsewhere — an
//     operator command, a job — and the module only gates the pool on a
//     no-op Ready, which keeps the graph shape identical across postures.
//
// The stages themselves stay where they are (setup.Setup,
// migrate.Migrator); this package owns only their ordering relative to the
// connection pool. See bringup.Ready for why that ordering must live in the
// dependency graph rather than in lifecycle hooks.
package gorm
