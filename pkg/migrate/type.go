package migrate

type Migrator interface {
	Migrate() error
}
