//go:build !darwin && !linux && !freebsd && !netbsd

package config

import (
	"os"
	"time"
)

// accessTime is unavailable on this platform (notably Windows), so callers
// fall back to the modification time.
func accessTime(os.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
