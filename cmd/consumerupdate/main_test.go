package main

import (
	"bytes"
	"context"
	"testing"
)

func TestRequiredFlags(t *testing.T) {
	for _, args := range [][]string{nil, {"unexpected"}, {"--event-file", "event.json"}, {"--unknown"}} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
