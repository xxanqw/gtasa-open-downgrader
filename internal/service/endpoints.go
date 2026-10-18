// Package service defines the first-party network service used by the desktop application.
package service

const (
	BaseURL            = "https://dwngrdr.xxanqw.me"
	HealthURL          = BaseURL + "/api/v1/health"
	LatestReleaseURL   = BaseURL + "/api/v1/releases/latest"
	LauncherReleaseURL = BaseURL + "/api/v1/launcher/latest"
	GitHubMetadataURL  = BaseURL + "/api/v1/github/repos"
)
