package gorm

import (
	"fmt"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"gorm.io/gorm"
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

// Setup initializes the database based on the specified parameters and the configured dialect in ownerConfig.
// Returns an error if the dialect is unsupported or if the setup process encounters an issue.
func (g *gormSetup) Setup(params setup.Params) error {

	switch g.ownerConfig.Dialect {
	case "postgres":
		return g.setupPostgres(params)
	case "sqlite":
		log.Info().Msg("no need to setup sqlite")
		return nil
	default:
		return errors.Errorf("unknown Dialect %q", g.ownerConfig.Dialect)
	}

}

// teardownPostgres removes the PostgreSQL database and user setup by the application.
func (g *gormSetup) teardownPostgres() error {
	if err := g.init(g.ownerConfig); err != nil {
		return err
	}

	log.Info().Interface("appConfig", g.appConfig).Msg("teardown")

	if err := g.dropDatabase(); err != nil {
		return err
	}
	if err := g.dropUser(); err != nil {
		return err
	}
	return nil
}

// Teardown cleans up resources based on the configured database dialect. Returns an error if the dialect is unknown.
func (g *gormSetup) Teardown() error {

	switch g.ownerConfig.Dialect {
	case "postgres":
		return g.teardownPostgres()
	case "sqlite":
		log.Info().Msg("no need to teardown sqlite")
		return nil
	default:
		return errors.Errorf("unknown Dialect %q", g.ownerConfig.Dialect)
	}
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
			tx = g.db.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", g.appConfig.Username, g.appConfig.Password))
			if tx.Error != nil {
				return tx.Error
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
	tx = g.db.Exec(fmt.Sprintf("DROP USER %s", g.appConfig.Username))
	if tx.Error != nil {
		return tx.Error
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

// createDatabase creates a new database using the given name from the appConfig configuration.
func (g *gormSetup) createDatabase() error {

	log.Info().Msg("creating database")

	tx := g.db.Exec(fmt.Sprintf("CREATE DATABASE %s", g.appConfig.Name))

	return tx.Error
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

	tx = g.db.Exec(fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND leader_pid IS NULL", g.appConfig.Name))
	if tx.Error != nil {
		return tx.Error
	}
	tx = g.db.Exec(fmt.Sprintf("DROP DATABASE %s", g.appConfig.Name))
	if tx.Error != nil {
		return tx.Error
	}
	log.Info().Msg("dropped database")

	return nil
}

// grantAllToDatabase grants all privileges on the specified database to the associated user defined in appConfig.
func (g *gormSetup) grantAllToDatabase() error {

	log.Info().Msg("granting all on database")
	tx := g.db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO  %s", g.appConfig.Name, g.appConfig.Username))
	return tx.Error

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
		tx := db.Exec(stmt)
		if tx.Error != nil {
			return tx.Error
		}
	}

	sDB, err := db.DB()

	if err != nil {
		return err
	}

	return sDB.Close()
}
