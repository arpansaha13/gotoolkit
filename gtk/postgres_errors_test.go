package gtk

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMapNoRows(t *testing.T) {
	t.Parallel()

	if MapNoRows(nil) != nil {
		t.Fatal("expected nil")
	}

	err := MapNoRows(pgx.ErrNoRows)
	if !errors.Is(err, &RecordNotFoundError{}) {
		t.Fatalf("got %v", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("pgx.ErrNoRows leaked")
	}

	other := errors.New("boom")
	if MapNoRows(other) != other {
		t.Fatal("expected original error")
	}
}
