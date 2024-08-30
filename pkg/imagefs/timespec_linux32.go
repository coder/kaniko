//go:build arm && linux

package imagefs

import (
	"syscall"
	"time"
)

func timespec(t time.Time) syscall.Timespec {
	return syscall.Timespec{Sec: int32(t.Unix()), Nsec: int32(t.Nanosecond())}
}
