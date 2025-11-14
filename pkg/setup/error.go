package setup

// NewResourceExistsError creates a new error indicating that a resource with the specified name already exists.
func NewResourceExistsError(name string) error {
	return ResourceExistsError{name}
}
