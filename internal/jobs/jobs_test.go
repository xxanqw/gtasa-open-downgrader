package jobs

import (
	"context"
	"testing"
	"time"
)

func TestShutdownCancelsAndWaits(t *testing.T) {
	g := New()
	started, finished := make(chan struct{}), make(chan struct{})
	g.Go(func(ctx context.Context) { close(started); <-ctx.Done(); close(finished) })
	<-started
	done := make(chan struct{})
	go func() { g.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel worker")
	}
	select {
	case <-finished:
	default:
		t.Fatal("closed before worker finished")
	}
	if g.Go(func(context.Context) { t.Error("work accepted after shutdown") }) {
		t.Fatal("group accepted late work")
	}
}
