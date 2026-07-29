//go:build linux

package config

import (
	"os"
	"syscall"
	"time"
)

func accessTime(fi os.FileInfo) (time.Time, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(st.Atim.Unix()), true
}
