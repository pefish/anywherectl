package version

import (
	"fmt"
	"runtime"
)

const AppName = "anywherectl"
const AppVersion = "v0.0.1"
const ProtocolVersion = "v1.0"

func GetAppVersion(app string) string {
	return fmt.Sprintf("%s %s (built w/%s)", app, AppVersion, runtime.Version())
}
