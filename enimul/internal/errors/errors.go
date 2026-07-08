package errors

import (
	"errors"

	F "github.com/lzpls/enimul/internal/fmt"
)

func New(text string) error { return errors.New(text) }

func NewAny(args ...any) error {
	return New(F.Concat(args...))
}

type opError struct {
	op  string
	err error
}

func (e *opError) Error() string {
	return e.op + ": " + e.err.Error()
}

func (e *opError) Unwrap() error {
	return e.err
}

func WithStr(op string, err error) error {
	if err == nil {
		return nil
	}
	return &opError{op: op, err: err}
}

func WithAny(err error, args ...any) error {
	if err == nil {
		return nil
	}
	return &opError{
		op:  F.Concat(args...),
		err: err,
	}
}
