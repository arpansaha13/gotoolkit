package gtk

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// RecordNotFoundError is returned when a query expects a row and none exists.
type RecordNotFoundError struct{}

func (e *RecordNotFoundError) Error() string { return "record not found" }

func (e *RecordNotFoundError) Is(target error) bool {
	_, ok := target.(*RecordNotFoundError)
	return ok
}

// NotConnectedError is returned when a query runs before Start or after Stop.
type NotConnectedError struct{}

func (e *NotConnectedError) Error() string { return "postgres client not connected" }

func (e *NotConnectedError) Is(target error) bool {
	_, ok := target.(*NotConnectedError)
	return ok
}

// MapNoRows maps a missing-row driver result to RecordNotFoundError.
func MapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return &RecordNotFoundError{}
	}
	return err
}
