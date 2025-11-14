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

type gormSetup struct {
	ownerConfig *datagorm.GormConfig
	appConfig   *datagorm.GormConfig
	db          *gorm.DB
}

type SetupParams struct {
	fx.In
	OwnerConfig *OwnerGormConfig
	AppConfig   *datagorm.GormConfig
}

func NewSetup(params SetupParams) setup.Setup {
	return &gormSetup{
		ownerConfig: &params.OwnerConfig.GormConfig,
		appConfig:   params.AppConfig,
	}
}

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

// Setup initializes the database setup process, including user creation, database creation, and granting required privileges.
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

// Teardown performs cleanup by dropping the database and user associated with the current configuration. Returns an error if any step fails.
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

func (g *gormSetup) init(cfg *datagorm.GormConfig) error {
	db, err := datagorm.NewDB(cfg)

	if err != nil {
		return err
	}
	g.db = db
	return nil
}

// PgRole represents a PostgreSQL role with the rolname field indicating the name of the role.
type PgRole struct {
	Rolname string
}

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

// PgDatabase represents a PostgreSQL database with its name.
type PgDatabase struct {
	Datname string
}

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

func (g *gormSetup) createDatabase() error {

	log.Info().Msg("creating database")

	tx := g.db.Exec(fmt.Sprintf("CREATE DATABASE %s", g.appConfig.Name))

	return tx.Error
}

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

func (g *gormSetup) grantAllToDatabase() error {

	log.Info().Msg("granting all on database")
	tx := g.db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO  %s", g.appConfig.Name, g.appConfig.Username))
	return tx.Error

}

func (g *gormSetup) grantAllToSchema() error {

	log.Info().Msg("granting schema permissions")
	db, err := datagorm.NewDB(&datagorm.GormConfig{
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
