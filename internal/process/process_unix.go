//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// verifyProcessName checks if the given PID belongs to a process with the expected name
func verifyProcessName(pid int, expectedName string) bool {
	if pid <= 0 {
		return false
	}

	// Try ps command first (most portable across Unix systems)
	// Use "args=" to get full command line instead of truncated comm
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err == nil {
		args := strings.TrimSpace(string(output))
		if strings.Contains(args, expectedName) {
			return true
		}
	}

	// On Linux, also try /proc/<pid>/exe (faster and more reliable when available)
	if runtime.GOOS == "linux" {
		exePath := fmt.Sprintf("/proc/%d/exe", pid)
		if link, err := os.Readlink(exePath); err == nil {
			baseName := filepath.Base(link)
			// Strip " (deleted)" suffix that appears when binary is replaced
			baseName = strings.TrimSuffix(baseName, " (deleted)")
			if strings.Contains(baseName, expectedName) {
				return true
			}
		}
	}

	return false
}

func terminateProcess(proc *os.Process) error {
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return nil
}

func stopAllDaemons(name string) error {
	if err := exec.Command("pkill", "-f", name).Run(); err != nil {
		if err := exec.Command("killall", name).Run(); err != nil {
			return fmt.Errorf("process: stop daemons: %w", err)
		}
	}
	return nil
}

func findProcessByName(name string) (int, error) {
	if output, err := exec.Command("pgrep", "-f", name).Output(); err == nil {
		pids := strings.Fields(string(output))
		for _, pidStr := range pids {
			pid, convErr := strconv.Atoi(strings.TrimSpace(pidStr))
			if convErr == nil {
				return pid, nil
			}
		}
	}

	psOutput, err := exec.Command("ps", "-eo", "pid,command").Output()
	if err != nil {
		return 0, fmt.Errorf("process: list processes: %w", err)
	}
	lines := strings.Split(string(psOutput), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, convErr := strconv.Atoi(fields[0])
		if convErr != nil {
			continue
		}
		if strings.Contains(line, name) {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("process: not found: %s", name)
}
