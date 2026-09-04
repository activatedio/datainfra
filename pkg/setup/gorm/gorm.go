package gorm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"gorm.io/gorm"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
)

// gormSetup is a type that facilitates setting up and tearing down Gorm-based database configurations and connections.
type gormSetup struct {
	ownerConfig *datagorm.Config
	appConfig   *datagorm.Config
	db          *gorm.DB
}

// SetupParams defines the parameters required to set up an application, including database configurations.
type SetupParams struct {
	fx.In
	OwnerConfig *OwnerGormConfig
	AppConfig   *datagorm.Config
}

// NewSetup creates and returns a new setup instance, initializing it with the provided SetupParams configuration.
func NewSetup(params SetupParams) setup.Setup {
	return &gormSetup{
		ownerConfig: &params.OwnerConfig.Config,
		appConfig:   params.AppConfig,
	}
}

// setupPostgres sets up a PostgreSQL database by initializing, checking existence, creating a user, database, and permissions.
func (g *gormSetup) setupPostgres(params setup.Params) error {

	if err := g.init(g.ownerConfig); err != nil {
		return err
	}

	defer g.closeOwnerDB()

	log.Info().Interface("appConfig", g.appConfig).Msg("setup")

	exists, name, err := g.databaseExists()

	if err != nil {
		return err
	}

	if exists {
		if params.FailOnExisting {
			return setup.NewResourceExistsError(name)
		}
		return nil
	}

	// When the app connects as the owner itself there is no separate role
	// to create and nothing to grant — self-grants are no-ops, and on
	// YugabyteDB every role/grant statement is a global-impact DDL that
	// bumps all databases' catalog versions (aborting concurrent
	// transactions with SQLSTATE 40001). Skipping them keeps CREATE
	// DATABASE as the only global DDL of a setup run.
	if g.appIsOwner() {
		log.Info().Msg("app user is the owner; skipping user creation and grants")
		return g.createDatabase()
	}

	if err = g.createUser(); err != nil {
		return err
	}
	if err = g.createDatabase(); err != nil {
		return err
	}
	if err = g.grantAllToDatabase(); err != nil {
		return err
	}
	if err = g.grantAllToSchema(); err != nil {
		return err
	}
	return nil
}

// appIsOwner reports whether the application connects as the owner role
// itself, in which case user creation, grants, and user teardown are
// skipped.
func (g *gormSetup) appIsOwner() bool {
	return g.appConfig.Username == g.ownerConfig.Username
}

// Setup initializes the database based on the specified parameters and the configured dialect in ownerConfig.
// Returns an error if the dialect is unsupported or if the setup process encounters an issue.
func (g *gormSetup) Setup(_ context.Context, params setup.Params) error {

	start := time.Now()
	var err error

	switch g.ownerConfig.Dialect {
	case "postgres":
		err = g.setupPostgres(params)
	case "sqlite":
		log.Info().Msg("no need to setup sqlite")
		return nil
	default:
		return errors.Errorf("unknown Dialect %q", g.ownerConfig.Dialect)
	}

	log.Info().Str("component", "gorm").Str("dialect", g.ownerConfig.Dialect).Str("database", g.appConfig.Name).Str("duration", time.Since(start).String()).Msg("db setup duration")
	return err

}

// teardownPostgres removes the PostgreSQL database and user setup by the application.
func (g *gormSetup) teardownPostgres() error {
	if err := g.init(g.ownerConfig); err != nil {
		return err
	}

	defer g.closeOwnerDB()

	log.Info().Interface("appConfig", g.appConfig).Msg("teardown")

	if err := g.dropDatabase(); err != nil {
		return err
	}
	// Never drop the owner role out from under ourselves when the app
	// connects as the owner (see appIsOwner in setupPostgres).
	if !g.appIsOwner() {
		if err := g.dropUser(); err != nil {
			return err
		}
	}
	return nil
}

// teardownSqlite deletes the database file, plus the -wal and -shm sidecars
// SQLite may have written beside it.
//
// This used to be a no-op ("no need to teardown sqlite"), which left every
// caller leaking a file per fixture: Setup does not create the file (gorm does,
// on first connect), but it is still the database this Setup was configured to
// manage, and Teardown's contract is to destroy that database. For postgres
// that means DROP DATABASE; for SQLite the database *is* the file, so removing
// it is the same operation. A consumer running thousands of fixtures
// accumulated hundreds of megabytes of /tmp files that nothing ever collected.
//
// A missing file is success, not an error — Teardown is routinely called
// before Setup to clear a dirty slate (see TestSetup_Success), and on a
// dialect where Setup is a no-op there may genuinely be nothing there.
func (g *gormSetup) teardownSqlite() error {

	name := g.appConfig.Name

	// In-memory databases have no file. Both the bare form and the URI form
	// ("file::memory:", "...?mode=memory") are in use in the wild, and
	// os.Remove on either would be meaningless at best.
	if name == "" || strings.Contains(name, ":memory:") || strings.Contains(name, "mode=memory") {
		log.Info().Msg("sqlite database is in-memory, nothing to remove")
		return nil
	}

	// A DSN can carry query parameters ("file.db?_pragma=..."); the file is
	// the part before them.
	path := name
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimPrefix(path, "file:")

	log.Info().Str("database", path).Msg("removing sqlite database file")

	// The sidecars are removed on a best-effort basis: their absence is the
	// normal case (they exist only while a connection is open in WAL mode),
	// so only the main file's failure is worth reporting.
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			log.Debug().Str("file", sidecar).Err(err).Msg("could not remove sqlite sidecar")
		}
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			log.Info().Str("database", path).Msg("sqlite database file does not exist, nothing to remove")
			return nil
		}
		return errors.Wrapf(err, "removing sqlite database %q", path)
	}

	return nil
}

// Teardown cleans up resources based on the configured database dialect. Returns an error if the dialect is unknown.
func (g *gormSetup) Teardown(_ context.Context) error {

	start := time.Now()
	var err error

	switch g.ownerConfig.Dialect {
	case "postgres":
		err = g.teardownPostgres()
	case "sqlite":
		err = g.teardownSqlite()
	default:
		return errors.Errorf("unknown Dialect %q", g.ownerConfig.Dialect)
	}

	log.Info().Str("component", "gorm").Str("dialect", g.ownerConfig.Dialect).Str("database", g.appConfig.Name).Str("duration", time.Since(start).String()).Msg("db teardown duration")
	return err
}

// init initializes the database connection using the provided configuration and assigns it to the gormSetup instance.
func (g *gormSetup) init(cfg *datagorm.Config) error {
	db, err := datagorm.NewDB(cfg)

	if err != nil {
		return err
	}
	g.db = db
	return nil
}

// closeOwnerDB releases the owner pool opened by init.
//
// Every Setup and every Teardown opens one, and until now none of them were
// closed: a process standing up many fixtures leaked a pool — and its idle
// connections — per call, against a server with a finite connection budget.
// Failure to close is logged rather than returned, since it cannot change
// whether the setup itself succeeded.
func (g *gormSetup) closeOwnerDB() {
	if g.db == nil {
		return
	}
	sDB, err := g.db.DB()
	if err != nil {
		log.Warn().Err(err).Msg("could not reach the owner pool to close it")
		return
	}
	if err := sDB.Close(); err != nil {
		log.Warn().Err(err).Msg("could not close the owner pool")
	}
	g.db = nil
}

// PgRole represents a PostgreSQL role, typically used to define database users or groups of users.
type PgRole struct {
	Rolname string
}

// createUser checks if a database user exists, and creates it with a password if it does not exist.
func (g *gormSetup) createUser() error {

	log.Info().Msg("creating user if it doesn't exist")

	tx := g.db.Table("pg_roles").Where("rolname = ?", g.appConfig.Username).First(&PgRole{})

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			log.Info().Msg("role not found, creating")
			if err := datagorm.ExecWithSerializationRetry(g.db, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", g.appConfig.Username, g.appConfig.Password)); err != nil {
				// The exists-check above is check-then-create: a concurrent
				// bring-up against the same database target can win the race
				// between the check and the CREATE. The role existing is the
				// goal, not a failure.
				if !datagorm.IsDuplicateObject(err) {
					return err
				}
				log.Info().Msg("role was created concurrently by another bring-up")
			}
			log.Info().Msg("created role")
		} else {
			return tx.Error
		}
	} else {
		log.Info().Msg("role already exists")
	}

	return nil

}

// dropUser removes a database user if it exists, logging the outcome and returning an error if any operation fails.
func (g *gormSetup) dropUser() error {

	log.Info().Str("user", g.appConfig.Username).Msg("creating user if it doesn't exist")

	tx := g.db.Table("pg_roles").Where("rolname = ?", g.appConfig.Username).First(&PgRole{})

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			log.Info().Msg("role not found, not dropping")
			return nil
		}
		return tx.Error
	}
	if err := datagorm.ExecWithSerializationRetry(g.db, fmt.Sprintf("DROP USER %s", g.appConfig.Username)); err != nil {
		return err
	}
	log.Info().Msg("dropped role")

	return nil

}

// PgDatabase represents a PostgreSQL database with a specific name.
// The Datname field contains the name of the database.
type PgDatabase struct {
	Datname string
}

// databaseExists checks if the specified database exists in the PostgreSQL instance and returns its existence status.
func (g *gormSetup) databaseExists() (bool, string, error) {

	log.Info().Msg("checking to see if database exists")

	tx := g.db.Table("pg_database").Where("datname = ?", g.appConfig.Name).First(&PgDatabase{})

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			log.Info().Msg("database does not exist")
			return false, "", nil
		}
		return false, "", tx.Error
	}
	log.Info().Msg("database exists")
	return true, g.appConfig.Name, nil

}

// databaseDDLAttempts bounds the create/drop retry loops below. CREATE/DROP
// DATABASE are not transactional on YugabyteDB, so a conflict can leave the
// keyspace half-created and being cleaned up asynchronously; the loop is
// waiting for that cleanup, which is quick but not instant.
const databaseDDLAttempts = 6

// databaseDDLBaseWait is the linear backoff step between attempts.
const databaseDDLBaseWait = 250 * time.Millisecond

// createDatabase creates a new database using the given name from the appConfig configuration.
//
// The loop exists because CREATE DATABASE is not transactional on
// YugabyteDB. A read restart (SQLSTATE 40001) can be reported *after* the
// DocDB keyspace has been created, with the catalog row rolled back —
// ExecWithSerializationRetry then re-issues the statement and gets
// "Keyspace 'x' already exists" (XX000) from the DocDB layer while
// pg_database still has no row for it. Neither error means what it appears
// to: the first is not "nothing happened" and the second is not "the
// database is ready".
//
// So every failure is resolved by looking at pg_database rather than by
// reading the error alone. The orphaned keyspace is reaped asynchronously,
// which is what the backoff waits for.
//
// Observed under `go test ./...` at default parallelism, where a dozen
// packages create their own test databases at once.
func (g *gormSetup) createDatabase() error {

	log.Info().Msg("creating database")

	var err error

	for attempt := 1; attempt <= databaseDDLAttempts; attempt++ {

		err = g.withDatabaseDDLLock(func() error {
			return datagorm.ExecWithSerializationRetry(g.db, fmt.Sprintf("CREATE DATABASE %s", g.appConfig.Name))
		})

		if err == nil {
			return nil
		}

		if datagorm.IsDuplicateObject(err) {
			// Setup's exists-check is check-then-create: a concurrent
			// bring-up against the same database target can create it
			// between the check and here. The database existing is the
			// goal, not a failure.
			log.Info().Msg("database was created concurrently by another bring-up")
			return nil
		}

		if !datagorm.IsNamespaceExists(err) {
			return err
		}

		// The keyspace exists but postgres may not know about it. Ask.
		exists, _, checkErr := g.databaseExists()

		if checkErr != nil {
			return checkErr
		}

		if exists {
			log.Info().Msg("database exists after a conflicting create; treating as created")
			return nil
		}

		log.Warn().
			Int("attempt", attempt).
			Err(err).
			Msg("keyspace exists but the database does not; waiting for the orphaned keyspace to be reaped")

		time.Sleep(time.Duration(attempt) * databaseDDLBaseWait)
	}

	return errors.Wrapf(err, "creating database %s: the keyspace kept reporting as existing while the database did not appear after %d attempts",
		g.appConfig.Name, databaseDDLAttempts)
}

// dropDatabase drops the specified database if it exists and terminates active connections to it. Returns an error if any operation fails.
func (g *gormSetup) dropDatabase() error {

	log.Info().Str("database", g.appConfig.Name).Msg("drop database if it exists")

	tx := g.db.Table("pg_database").Where("datname = ?", g.appConfig.Name).First(&PgDatabase{})

	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			log.Info().Msg("database does not exist, not dropping")
			return nil
		}
		return tx.Error
	}
	log.Info().Msg("database exists, dropping")

	var err error

	// Mirror of createDatabase, for the same reason: DROP DATABASE is not
	// transactional on YugabyteDB, so a conflict can be reported after the
	// drop has taken effect. Re-issuing then reports the database as absent,
	// which is the outcome the drop wanted, and a conflict that did NOT take
	// effect is worth another attempt — so the state, not the error, decides.
	for attempt := 1; attempt <= databaseDDLAttempts; attempt++ {

		err = g.withDatabaseDDLLock(func() error {
			// Terminate inside the lock and immediately before the drop:
			// re-run per attempt because a client that reconnected between
			// attempts would otherwise hold the drop off indefinitely.
			if tx := g.db.Exec(fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND leader_pid IS NULL", g.appConfig.Name)); tx.Error != nil {
				return tx.Error
			}
			return datagorm.ExecWithSerializationRetry(g.db, fmt.Sprintf("DROP DATABASE %s", g.appConfig.Name))
		})

		if err == nil {
			log.Info().Msg("dropped database")
			return nil
		}

		if datagorm.IsUndefinedObject(err) {
			log.Info().Msg("database was already gone")
			return nil
		}

		exists, _, checkErr := g.databaseExists()

		if checkErr != nil {
			return checkErr
		}

		if !exists {
			log.Info().Msg("database is gone after a conflicting drop; treating as dropped")
			return nil
		}

		log.Warn().
			Int("attempt", attempt).
			Err(err).
			Msg("drop database conflicted and the database is still present; retrying")

		time.Sleep(time.Duration(attempt) * databaseDDLBaseWait)
	}

	return errors.Wrapf(err, "dropping database %s after %d attempts", g.appConfig.Name, databaseDDLAttempts)
}

// grantAllToDatabase grants all privileges on the specified database to the associated user defined in appConfig.
func (g *gormSetup) grantAllToDatabase() error {

	log.Info().Msg("granting all on database")
	return datagorm.ExecWithSerializationRetry(g.db, fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO  %s", g.appConfig.Name, g.appConfig.Username))

}

// grantAllToSchema grants all necessary schema privileges to the specified user for the database schema.
func (g *gormSetup) grantAllToSchema() error {

	log.Info().Msg("granting schema permissions")
	db, err := datagorm.NewDB(&datagorm.Config{
		Dialect:          g.ownerConfig.Dialect,
		Host:             g.ownerConfig.Host,
		Port:             g.ownerConfig.Port,
		Username:         g.ownerConfig.Username,
		Password:         g.ownerConfig.Password,
		Name:             g.appConfig.Name,
		EnableSQLLogging: true,
	})

	if err != nil {
		return err
	}

	// TODO - for now we do this, eventually we want something more granular
	stmts := []string{
		fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", g.appConfig.Username),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO %s", g.appConfig.Username),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO %s", g.appConfig.Username),
	}

	for _, stmt := range stmts {
		if err := datagorm.ExecWithSerializationRetry(db, stmt); err != nil {
			return err
		}
	}

	sDB, err := db.DB()

	if err != nil {
		return err
	}

	return sDB.Close()
}
