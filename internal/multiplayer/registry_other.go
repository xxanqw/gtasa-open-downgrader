//go:build !windows

package multiplayer

func registryCandidates() map[ClientID][]string  { return nil }
func mtaRegistryCandidates() map[string][]string { return nil }
