package cacheinstance

import "errors"

var (
	ErrRegistryClosed   = errors.New("registry closed")
	ErrKindExists       = errors.New("kind exists")
	ErrKindNotFound     = errors.New("kind not found")
	ErrInstanceExists   = errors.New("instance exists")
	ErrInstanceNotFound = errors.New("instance not found")
	ErrFactoryNotFound  = errors.New("factory not found")
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrInvalidJSON      = errors.New("invalid JSON")
	ErrInvalidInstance  = errors.New("invalid instance")
	ErrTypeMismatch     = errors.New("type mismatch")
)
