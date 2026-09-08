// multiplayer-context is a developer diagnostic, not an end-user interface.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

func main() {
	gameDir := flag.String("game", "", "GTA San Andreas directory")
	flag.Parse()
	if *gameDir == "" {
		fmt.Fprintln(os.Stderr, "use -game with the GTA San Andreas directory")
		os.Exit(2)
	}
	launcher := entrypoint.Inspect(context.Background(), *gameDir)
	environment, _, compatibility, err := multiplayer.PlatformEnvironment(*gameDir)
	if environment.GameDir == "" {
		environment.GameDir = *gameDir
	}
	fmt.Printf("Game: %s\n", environment.GameDir)
	fmt.Printf("Launcher: %s", launcher.State)
	if launcher.Detail != "" {
		fmt.Printf(" (%s)", launcher.Detail)
	}
	fmt.Println()
	if compatibility != "" {
		fmt.Printf("Steam compatibility: %s\n", compatibility)
	}
	if err != nil {
		fmt.Printf("Environment: %v\n", err)
	}
	for _, client := range multiplayer.Detect(environment) {
		value := "not installed"
		if client.Installed {
			value = client.Executable
		}
		fmt.Printf("%s: %s\n", client.Name, value)
	}
}
