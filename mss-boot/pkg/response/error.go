package response

import (
	"fmt"
)

type Error interface {
	error
	ErrorCode() string
	ErrorMsg() string
}

type DefaultError struct {
	Code string `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
}

func (e *DefaultError) ErrorCode() string {
	return e.Code
}

func (e *DefaultError) ErrorMsg() string {
	return e.Msg
}

func (e *DefaultError) Error() string {
	return fmt.Sprintf("code: %s, msg: %s", e.ErrorCode(), e.ErrorMsg())
}

func NewError(code, msg string) Error {
	return &DefaultError{Code: code, Msg: msg}
}
