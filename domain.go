package gotoolkit

import (
	internaldomain "github.com/arpansaha13/gotoolkit/internal/domain"
)

// Domain error types (type aliases — same type across package boundaries)
type ValidationError = internaldomain.ValidationError
type ConflictError = internaldomain.ConflictError
type NotFoundError = internaldomain.NotFoundError
type UnauthorizedError = internaldomain.UnauthorizedError
type ForbiddenError = internaldomain.ForbiddenError
type InternalError = internaldomain.InternalError

// Domain error type checkers
var (
	IsValidation   = internaldomain.IsValidation
	IsConflict     = internaldomain.IsConflict
	IsNotFound     = internaldomain.IsNotFound
	IsUnauthorized = internaldomain.IsUnauthorized
	IsForbidden    = internaldomain.IsForbidden
)
