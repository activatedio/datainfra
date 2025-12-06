package gorm

import (
	"context"
	"fmt"

	"github.com/activatedio/datainfra/pkg/data"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// AssociateParams is a generic type used to manage associations between a parent entity and child entities in a database.
type AssociateParams[PK comparable, CK comparable] struct {
	ParentKey        PK
	Add              []CK
	Remove           []CK
	ParentRepository data.AssociateParentRepository[PK]
	ChildRepository  data.AssociateChildRepository[CK]
	AssociationTable string
	ParentColumnName string
	ChildColumnName  string
	ExecuteRemove    func(ctx context.Context, db *gorm.DB, params AssociateParams[PK, CK], remove []CK) *gorm.DB
	ExecuteAdd       func(ctx context.Context, db *gorm.DB, params AssociateParams[PK, CK], add CK) *gorm.DB
}

// Associate manages the association of a parent entity with child entities, adding or removing as specified in the parameters.
func Associate[PK comparable, CK comparable](ctx context.Context, params AssociateParams[PK, CK]) error {
	parentExists, err := params.ParentRepository.ExistsByKey(ctx, params.ParentKey)
	if err != nil {
		return err
	}
	if !parentExists {
		return errors.New("parent key not found")
	}

	filteredAdd, err := params.ChildRepository.FilterKeys(ctx, params.Add)
	if err != nil {
		return err
	}
	filteredRemove, err := params.ChildRepository.FilterKeys(ctx, params.Remove)
	if err != nil {
		return err
	}

	tx := GetDB(ctx)

	if isNotEmpty(filteredRemove) {
		if err := executeRemove(ctx, tx, params, filteredRemove); err != nil {
			return err
		}
	}

	for _, childKey := range filteredAdd {
		if err := executeAdd(ctx, tx, params, childKey); err != nil {
			return err
		}
	}

	return nil
}

func isNotEmpty[CK any](keys []CK) bool {
	return len(keys) > 0
}

func executeRemove[PK comparable, CK comparable](ctx context.Context, tx *gorm.DB, params AssociateParams[PK, CK], keysToRemove []CK) error {
	if params.ExecuteRemove != nil {
		tx = params.ExecuteRemove(ctx, tx, params, keysToRemove)
	} else {
		tx = tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND %s IN ?",
			params.AssociationTable, params.ParentColumnName, params.ChildColumnName),
			params.ParentKey, keysToRemove)
	}
	return tx.Error
}

func executeAdd[PK comparable, CK comparable](ctx context.Context, tx *gorm.DB, params AssociateParams[PK, CK], keyToAdd CK) error {
	if params.ExecuteAdd != nil {
		tx = params.ExecuteAdd(ctx, tx, params, keyToAdd)
	} else {
		tx = tx.Exec(fmt.Sprintf("INSERT INTO %s (%s, %s, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)",
			params.AssociationTable, params.ParentColumnName, params.ChildColumnName),
			params.ParentKey, keyToAdd)
	}
	return tx.Error
}
