package collector

import "fmt"

type notImplError struct {
	module string
}

func (e *notImplError) Error() string {
	return fmt.Sprintf("collector %q: no platform implementation registered", e.module)
}

func errNotImplemented(module string) error {
	return &notImplError{module: module}
}
