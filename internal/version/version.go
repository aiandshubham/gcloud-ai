package version
 
import "fmt"
 
// These are set at build time via ldflags:
// -X gcloud-ai/internal/version.Version=v1.0.0
// -X gcloud-ai/internal/version.Commit=abc1234
// -X gcloud-ai/internal/version.BuildDate=2025-03-25
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
 
func Print() {
	fmt.Printf("gcloud-ai %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
}
 
func String() string {
	return Version
}
