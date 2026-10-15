//go:build windows

package integration_test

import "syscall"

func capacityCPUSeconds() (float64, error) {
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, err
	}
	var created, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(handle, &created, &exit, &kernel, &user); err != nil {
		return 0, err
	}
	ticks := func(value syscall.Filetime) uint64 { return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime) }
	return float64(ticks(kernel)+ticks(user)) / 10_000_000, nil
}
