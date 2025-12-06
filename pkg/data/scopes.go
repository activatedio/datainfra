package data

import "context"

// FetchType represents a type of fetch operation, used to influence context-specific behaviors or query logic.
type FetchType string

const (

	// FetchTypeKeys fetch of only keys
	FetchTypeKeys = "KEYS"
	// FetchTypeList fetch of a list
	FetchTypeList = "LIST"
	// FetchTypeDetail fetch of a detail of an item
	FetchTypeDetail = "DETAIL"
	// FetchTypeNone no fetch type
	FetchTypeNone = "NONE"
)

// RootInfo represents a special type of context scope that represents no constraining scope.
type RootInfo struct{}

// NoneScopeTemplate defines a specialization of ScopeTemplate with RootInfo as the scope, representing no constraining scope.
type NoneScopeTemplate interface {
	ScopeTemplate[*RootInfo]
}

// MustGetRootInfo retrieves the RootInfo object from the provided context, representing no constraining scope.
func MustGetRootInfo(_ context.Context) *RootInfo {
	return &RootInfo{}
}
