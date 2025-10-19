//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

func daemonProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func daemonSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
