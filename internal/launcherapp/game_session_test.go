package launcherapp

import (
	"context"
	"errors"
	"testing"
)

func TestTrackedGameCleanupStillWaitsAndFailsClosed(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		started bool
		waitErr error
		cleanup bool
		wantErr bool
	}{
		{"GTA exited", true, nil, true, false},
		{"bootstrap failed without GTA", false, nil, true, true},
		{"cannot establish safe exit", false, errors.New("enumeration failed"), false, true},
		{"cancelled tracking", true, context.Canceled, false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cleaned := false
			runtime := &Runtime{GameDir: t.TempDir(), snapshotGameProcesses: func() (gameProcessSet, error) { return gameProcessSet{}, nil }, waitForGameProcess: func(_ context.Context, before gameProcessSet, done <-chan struct{}) (bool, error) {
				<-done
				if len(before) != 0 {
					t.Fatal("pre-launch snapshot should have no existing GTA processes")
				}
				if cleaned {
					t.Fatal("cleanup ran before the process waiter")
				}
				return scenario.started, scenario.waitErr
			}}
			err := runtime.runWithGameCleanup(context.Background(), func() error { return errors.New("bootstrap status") }, func() error { cleaned = true; return nil })
			if cleaned != scenario.cleanup || (err != nil) != scenario.wantErr {
				t.Fatal("unsafe cleanup or wrong launch result", cleaned, err)
			}
		})
	}
}
func TestGameCleanupSnapshotFailureDoesNotLaunch(t *testing.T) {
	runtime := &Runtime{snapshotGameProcesses: func() (gameProcessSet, error) { return nil, errors.New("enumeration failed") }}
	ran, cleaned := false, false
	err := runtime.runWithGameCleanup(context.Background(), func() error { ran = true; return nil }, func() error { cleaned = true; return nil })
	if err == nil || ran || cleaned {
		t.Fatal("process enumeration failure permitted launch/cleanup", err)
	}
}
