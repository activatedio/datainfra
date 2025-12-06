package migrate

// Migrator is an interface for migrating data
type Migrator interface {
	Migrate() error
}
