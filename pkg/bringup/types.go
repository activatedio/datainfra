package bringup

// Ready marks that database bring-up — setup and migrations, as configured —
// has completed.
//
// It exists to be depended on: connection constructors take *Ready so a DI
// graph cannot order a pool dial before bring-up finishes. The ordering is
// load-bearing rather than cosmetic — gorm's postgres driver connects
// eagerly, and fx runs every constructor before any OnStart hook, so
// bring-up registered as a lifecycle hook runs after the application pool
// has already tried to dial. On a deployment whose application role is
// created during setup, that is a boot failure with an authentication error.
type Ready struct{}
