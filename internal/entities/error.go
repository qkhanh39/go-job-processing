package entities

import "errors"

var (
	ErrInternalServer     = errors.New("An internal server error occurs")
	ErrUnsupportedJobType = errors.New("This job type is unsupported")
	ErrInvalidPayload     = errors.New("Invalid payload")
	ErrInvalidJson        = errors.New("Invalid Json")
)
