package migrate

import "context"

// Migrator is an interface for migrating data
type Migrator interface {
	Migrate(ctx context.Context) error
}
