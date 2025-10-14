//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
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
