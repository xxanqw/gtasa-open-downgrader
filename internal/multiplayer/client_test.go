package multiplayer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func file(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("MZtest"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectSupportedClients(t *testing.T) {
	game, driveC := t.TempDir(), t.TempDir()
	file(t, filepath.Join(game, "samp.exe"))
	file(t, filepath.Join(game, "samp.dll"))
	file(t, filepath.Join(driveC, "Program Files (x86)", "Open Multiplayer Launcher", "omp-launcher.exe"))
	mta := filepath.Join(t.TempDir(), "MTA", "Multi Theft Auto.exe")
	file(t, mta)
	clients := detect(Environment{GameDir: game, PrefixDriveC: driveC}, map[ClientID][]string{ClientMTA: {mta}})
	for _, id := range []ClientID{ClientSAMP, ClientOpenMP, ClientMTA} {
		client, ok := Find(clients, id)
		if !ok || !client.Installed || client.Executable == "" {
			t.Fatalf("%s not detected: %#v", id, client)
		}
	}
}

func TestSAMPRequiresDLLAndMissingExecutableIsRejected(t *testing.T) {
	game := t.TempDir()
	file(t, filepath.Join(game, "samp.exe"))
	client, _ := Find(detect(Environment{GameDir: game}, nil), ClientSAMP)
	if client.Installed {
		t.Fatal("incomplete SA-MP installation detected")
	}
	if err := ValidateExecutable(client); err == nil {
		t.Fatal("missing client executable accepted")
	}
}

func TestRunnersUseInjectedExecution(t *testing.T) {
	installer := filepath.Join(t.TempDir(), "setup.exe")
	file(t, installer)
	var direct []string
	runner := DirectRunner{Executor: CommandExecutorFunc(func(_ context.Context, dir, exe string, args ...string) error {
		direct = append([]string{dir, exe}, args...)
		return nil
	})}
	if err := runner.Run(context.Background(), installer, "/test"); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Dir(installer), installer, "/test"}
	if !reflect.DeepEqual(direct, want) {
		t.Fatalf("direct = %#v, want %#v", direct, want)
	}
	proton := &fakeProton{}
	if err := (ProtonRunner{Context: proton}).Run(context.Background(), installer, "/test"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(proton.call, []string{installer, "/test"}) {
		t.Fatalf("Proton call = %#v", proton.call)
	}
}

func TestInstallerPreparationProvidesAccessibleGamePath(t *testing.T) {
	environment := Environment{GameDir: "/home/user/.steam/game", WindowsGameDir: `Z:\home\user\.steam\game`}
	arguments, err := (DirectRunner{}).Prepare(context.Background(), ClientSAMP, environment)
	if err != nil || !reflect.DeepEqual(arguments, []string{`/D=Z:\home\user\.steam\game`}) {
		t.Fatalf("SA-MP arguments = %#v, %v", arguments, err)
	}
	prepared := false
	runner := ProtonRunner{Context: &fakeProton{}, PrepareFunc: func(_ context.Context, id ClientID, got Environment) error {
		prepared = id == ClientMTA && got.WindowsGameDir == environment.WindowsGameDir
		return nil
	}}
	arguments, err = runner.Prepare(context.Background(), ClientMTA, environment)
	if err != nil || len(arguments) != 0 || !prepared {
		t.Fatalf("MTA preparation = %#v, %v, prepared=%t", arguments, err, prepared)
	}
}

type fakeProton struct{ call []string }

func (f *fakeProton) Run(_ context.Context, executable string, arguments ...string) error {
	f.call = append([]string{executable}, arguments...)
	return nil
}

func TestInstallerValidation(t *testing.T) {
	valid := filepath.Join(t.TempDir(), "valid.exe")
	file(t, valid)
	if err := ValidateInstaller(valid); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.exe")
	if err := os.WriteFile(bad, []byte("HTML"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateInstaller(bad); err == nil {
		t.Fatal("non-PE installer accepted")
	}
	for _, raw := range []string{"http://example.com/a.exe", "file:///tmp/a.exe", "https:///a.exe"} {
		if err := ValidateDownloadURL(raw); err == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
	if err := ValidateDownloadURL(Installers[ClientOpenMP].URL); err != nil {
		t.Fatal(err)
	}
}

func TestMTAInstallerAcceptsVerifiedProtonCompletionCode(t *testing.T) {
	if os.Getenv("GO_TEST_EXIT_199") == "1" {
		os.Exit(199)
	}
	driveC := t.TempDir()
	mta := filepath.Join(driveC, "Program Files (x86)", "MTA San Andreas 1.6", "Multi Theft Auto.exe")
	file(t, mta)
	command := exec.Command(os.Args[0], "-test.run=TestMTAInstallerAcceptsVerifiedProtonCompletionCode")
	command.Env = append(os.Environ(), "GO_TEST_EXIT_199=1")
	runErr := command.Run()
	if runErr == nil {
		t.Fatal("helper process did not return exit code 199")
	}
	environment := Environment{GameDir: t.TempDir(), PrefixDriveC: driveC}
	if err := InstallerResult(ClientMTA, environment, fmt.Errorf("wrapped: %w", runErr)); err != nil {
		t.Fatalf("verified MTA installation was rejected: %v", err)
	}
	if err := InstallerResult(ClientMTA, Environment{GameDir: t.TempDir()}, runErr); err == nil {
		t.Fatal("exit code 199 was accepted without an installed client")
	}
	if err := InstallerResult(ClientSAMP, environment, runErr); err == nil {
		t.Fatal("exit code 199 was accepted for another installer")
	}
}

func TestClientConfigParsingAndPathValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"clients":{"openmp":{"executable":"portable/omp-launcher.exe"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	file(t, filepath.Join(root, "portable", "omp-launcher.exe"))
	config, err := LoadConfig(root)
	if err != nil || config.SchemaVersion != ConfigSchema {
		t.Fatalf("config = %#v, %v", config, err)
	}
	client, _ := Find(Detect(Environment{GameDir: root}), ClientOpenMP)
	if !client.Installed {
		t.Fatal("configured relative client was not detected")
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"clients":{"mta":{"executable":"../outside.exe"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(root); err == nil {
		t.Fatal("unsafe configured path accepted")
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(root); err == nil {
		t.Fatal("future config schema accepted")
	}
}
