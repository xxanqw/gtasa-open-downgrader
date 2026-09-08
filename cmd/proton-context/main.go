// Command proton-context is a developer diagnostic for Steam/Proton discovery.
package main

import (
	"fmt"
	"os"

	"github.com/xxanqw/gtasa-open-downgrader/internal/steam"
)

func main() {
	context, err := steam.DetectProtonContext(12120)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Steam root:", context.SteamRoot)
	fmt.Println("Library:", context.LibraryRoot)
	fmt.Println("Game directory:", context.GameDir)
	fmt.Println("Compatdata:", context.CompatDataPath)
	fmt.Printf("Compatibility tool: %s (%s; %s)\n", context.ToolDisplayName, context.ToolName, context.Selection)
	fmt.Println("Proton executable:", context.ProtonExecutable)
}
