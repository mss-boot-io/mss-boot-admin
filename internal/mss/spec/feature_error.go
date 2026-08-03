package spec

import "strings"

type validationError struct {
	Problems []string
}

func (e validationError) Error() string {
	return strings.Join(e.Problems, "; ")
}
