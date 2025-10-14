//go:build windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Query tasklist for the pid
	output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	lines := strings.Split(string(output), "\n")
	return len(lines) > 1 && strings.TrimSpace(lines[1]) != ""
}

func terminateProcess(proc *os.Process) error {
	return proc.Kill()
}

func stopAllDaemons(name string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", name+".exe")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("taskkill", "/F", "/IM", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("process: stop daemons: %w", err)
		}
	}
	return nil
}

func findProcessByName(name string) (int, error) {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV")
		output, err = cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("process: tasklist failed: %w", err)
		}
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\",\"")
		if len(fields) < 2 {
			continue
		}
		pidStr := strings.Trim(fields[1], "\"")
		pid, convErr := strconv.Atoi(pidStr)
		if convErr == nil {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("process: not found: %s", name)
}
