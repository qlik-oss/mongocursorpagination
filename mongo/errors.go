package mongo

import (
	"fmt"
)

type (
	ErrInvalidResults struct {
		message string
	}
)

func NewErrInvalidResults(message string) error {
	return &ErrInvalidResults{message: message}
}

func (e *ErrInvalidResults) Error() string {
	return e.message
}

type (
	ErrPaginatedFieldNotFound struct {
		fieldName string
	}
)

func NewErrPaginatedFieldNotFound(fieldName string) error {
	return &ErrPaginatedFieldNotFound{fieldName: fieldName}
}

func (e *ErrPaginatedFieldNotFound) Error() string {
	return fmt.Sprintf("paginated field %s not found", e.fieldName)
}
