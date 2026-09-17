package gorm

import (
	"context"
	"fmt"

	gorm1 "gorm.io/gorm"

	"github.com/activatedio/datainfra/pkg/data/gorm"
)

// ProductTagExecuteAdd inserts a product_tags edge row idempotently, where the
// generator's default add would fail the second time on the primary key.
//
// It is declared on the Tag edge's gorm.Associate and on no other, so it is
// also what proves the generator selects hooks per edge: AssociateTags calls
// it, AssociateCategories still uses the default.
func ProductTagExecuteAdd(_ context.Context, db *gorm1.DB, params gorm.AssociateParams[string, string],
	add string) *gorm1.DB {
	return db.Exec(fmt.Sprintf(
		"INSERT INTO %s (%s, %s, created_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT DO NOTHING",
		params.AssociationTable, params.ParentColumnName, params.ChildColumnName),
		params.ParentKey, add)
}
