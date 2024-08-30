//go:build !arm && linux

package imagefs

import (
	"syscall"
	"time"
)

func timespec(t time.Time) syscall.Timespec {
	return syscall.Timespec{Sec: t.Unix(), Nsec: int64(t.Nanosecond())}
}
