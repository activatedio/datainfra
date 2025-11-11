package data

// EntityAlreadyExists represents an error indicating that the entity being created already exists in the repository.
type EntityAlreadyExists struct {
}

// Error returns a string message indicating the entity already exists.
func (e EntityAlreadyExists) Error() string {
	return "entity already exists"
}
