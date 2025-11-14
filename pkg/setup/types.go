package setup

// ResourceExistsError represents an error indicating a resource with the specified name already exists.
type ResourceExistsError struct {
	name string
}

// Error returns a descriptive error message indicating that a resource with the specified name already exists.
func (r ResourceExistsError) Error() string {
	return "resource already exists: " + r.name
}

// Params represents configuration options
type Params struct {
	// FailOnExisting determines whether the operation should fail if the environment already exists
	FailOnExisting bool
}

// Setup defines methods to initialize and teardown environments for testing.
type Setup interface {
	// Setup initializes the environment, such as databases, and returns an error if setup fails.
	Setup(params Params) error
	// Teardown cleans up the environment, reversing setup processes, and returns an error if teardown fails.
	Teardown() error
}
