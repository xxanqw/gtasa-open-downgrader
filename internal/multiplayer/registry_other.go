//go:build !windows

package multiplayer

func registryCandidates() map[ClientID][]string { return nil }
