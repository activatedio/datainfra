package data

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

// MustGetRootInfo returns the canonical RootInfo value representing no
// constraining scope. The name preserves the historical signature; the
// function does not actually panic and takes no context, since RootInfo
// carries no per-call state.
func MustGetRootInfo() *RootInfo {
	return &RootInfo{}
}
