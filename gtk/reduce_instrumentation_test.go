package gtk

import (
	"context"
	"testing"
)

func TestReduceInstrumentation(t *testing.T) {
	if ReduceInstrumentation(nil) {
		t.Fatal("nil ctx must not reduce")
	}
	if ReduceInstrumentation(context.Background()) {
		t.Fatal("background must not reduce")
	}
	if !ReduceInstrumentation(WithReduceInstrumentation(context.Background())) {
		t.Fatal("WithReduceInstrumentation must reduce")
	}
	if !ReduceInstrumentation(WithReduceInstrumentation(nil)) {
		t.Fatal("WithReduceInstrumentation(nil) must reduce")
	}
}
