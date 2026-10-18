//go:build integration

package mods

import (
	"context"
	"testing"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
)

// This opt-in check downloads official packages only into a disposable game
// directory. It is separate from deterministic, network-free default tests.
func TestOfficialPackages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := download.New()
	resolver := NewResolver(client)
	installer := Installer{Client: client}
	for _, source := range Catalog {
		t.Run(source.Name, func(t *testing.T) {
			root := t.TempDir() // Mutually overlapping packages are audited separately.
			for _, dep := range RequiredMods(source.Name) {
				rel, err := resolver.Resolve(ctx, dep)
				if err != nil {
					t.Fatal(err)
				}
				if err = installer.install(ctx, resolver, root, dep, rel, nil, false); err != nil {
					t.Fatal(err)
				}
			}
			release, err := resolver.Resolve(ctx, source.Name)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Official version: %s", release.Version)
			if err = installer.install(ctx, resolver, root, source.Name, release, nil, false); err != nil {
				t.Fatal(err)
			}
		})
	}
}
