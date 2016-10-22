package sarah

/*
AdapterNonContinuableError represents critical error that Adapter can't continue its operation.
When Runner receives this, Runner must stop corresponding adapter.
*/
type AdapterNonContinuableError struct {
	err string
}

/*
Error returns detailed error about Adapter's non-continuable state.
*/
func (e AdapterNonContinuableError) Error() string {
	return e.err
}

/*
NewAdapterNonContinuableError creates and return new AdapterNonContinuableError instance.
*/
func NewAdapterNonContinuableError(errorContent string) error {
	return &AdapterNonContinuableError{err: errorContent}
}
