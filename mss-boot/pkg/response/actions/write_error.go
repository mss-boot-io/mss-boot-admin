package actions

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
)

// WriteOperation identifies the control boundary that produced an error.
type WriteOperation string

const (
	WriteOperationLoad         WriteOperation = "load"
	WriteOperationBeforeCreate WriteOperation = "before-create"
	WriteOperationCreate       WriteOperation = "create"
	WriteOperationCreator      WriteOperation = "creator"
	WriteOperationAfterCreate  WriteOperation = "after-create"
	WriteOperationAfterCommit  WriteOperation = "after-commit-create"
	WriteOperationBeforeUpdate WriteOperation = "before-update"
	WriteOperationUpdate       WriteOperation = "update"
	WriteOperationAfterUpdate  WriteOperation = "after-update"
)

// PublicWriteError is a value-safe HTTP classification. Error must not retain
// driver messages, SQL, credentials, or request identity values.
type PublicWriteError struct {
	Status int
	Error  response.Error
}

// WriteErrorMapper maps a provider/domain write error to a fixed public error.
// Returning false uses the generic redacted failure whenever a mapper is
// installed; raw causes are retained only by legacy controls with no mapper.
type WriteErrorMapper func(*gin.Context, WriteOperation, error) (PublicWriteError, bool)
