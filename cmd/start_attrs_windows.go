//go:build windows

package cmd

import (
	"os"
	"syscall"
)

func daemonProcAttr() *syscall.SysProcAttr {
	return nil
}

func daemonSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
