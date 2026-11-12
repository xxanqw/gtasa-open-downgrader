package launcherapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
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

func TestSinglePlayerFinishesWhenGameExitsBeforeLaunchCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	commandStarted := make(chan struct{})
	releaseCommand := make(chan struct{})
	commandExited := make(chan struct{})
	defer func() { close(releaseCommand); <-commandExited }()
	gameExited := make(chan struct{})
	runtime := &Runtime{GameDir: root,
		snapshotGameProcesses: func() (gameProcessSet, error) { return gameProcessSet{}, nil },
		Execute: func(context.Context, string, string, ...string) error {
			close(commandStarted)
			<-releaseCommand
			close(commandExited)
			return nil
		},
		waitForGameProcess: func(ctx context.Context, _ gameProcessSet, _ <-chan struct{}) (bool, error) {
			<-commandStarted
			select {
			case <-gameExited:
				return true, nil
			case <-ctx.Done():
				return true, ctx.Err()
			}
		},
	}
	result := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { result <- runtime.SinglePlayer(ctx) }()
	<-commandStarted
	select {
	case err := <-result:
		t.Fatalf("returned before GTA exited: %v", err)
	default:
	}
	close(gameExited)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("singleplayer still waiting for launch command after GTA exited")
	}
}

func TestSinglePlayerDistinguishesObservedShutdownFromStartupFailure(t *testing.T) {
	for _, started := range []bool{true, false} {
		t.Run(fmt.Sprintf("observed=%t", started), func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
				t.Fatal(err)
			}
			shutdown := errors.New("exit status 0xc0000409")
			runtime := &Runtime{GameDir: root,
				snapshotGameProcesses: func() (gameProcessSet, error) { return gameProcessSet{}, nil },
				Execute:               func(context.Context, string, string, ...string) error { return shutdown },
				waitForGameProcess: func(_ context.Context, _ gameProcessSet, done <-chan struct{}) (bool, error) {
					<-done
					return started, nil
				},
			}
			err := runtime.SinglePlayer(context.Background())
			if started && err != nil {
				t.Fatalf("observed shutdown reported as launch failure: %v", err)
			}
			if !started && !errors.Is(err, shutdown) {
				t.Fatalf("startup failure hidden: %v", err)
			}
		})
	}
}
