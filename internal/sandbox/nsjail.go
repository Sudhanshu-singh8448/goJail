package sandbox

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/nsjail-server/gojail/internal/config"
)

// buildNsjailArgs constructs the nsjail command line for a build or run phase.
// This replaces the old template-based approach with programmatic arg building.
func buildNsjailArgs(nsjailPath string, workDir string, limits config.Limits, isCompile bool) []string {
	args := []string{nsjailPath}

	// Mode
	args = append(args, "--mode", "once")

	// Chroot setup: bind-mount essential directories read-only
	args = append(args,
		"--bindmount_ro", "/bin",
		"--bindmount_ro", "/usr",
		"--bindmount_ro", "/lib",
		"--bindmount_ro", "/dev",
		"--bindmount_ro", "/etc",
	)

	// /lib64 may not exist on all systems, bind it if present
	if _, err := os.Stat("/lib64"); err == nil {
		args = append(args, "--bindmount_ro", "/lib64")
	}

	// Bind-mount the working directory read-write
	args = append(args, "--bindmount", workDir+":"+workDir)

	// Working directory inside the jail
	args = append(args, "--cwd", workDir)

	// Resource limits
	args = append(args, "--time_limit", strconv.Itoa(limits.WallTimeS))
	args = append(args, "--rlimit_as", strconv.Itoa(limits.MemoryKB))
	args = append(args, "--rlimit_nproc", strconv.Itoa(limits.MaxProcesses))
	args = append(args, "--rlimit_fsize", "100")     // 100 MB file size limit
	args = append(args, "--rlimit_nofile", "1000")

	// Environment
	args = append(args, "--env", "TMP=/tmp")
	args = append(args, "--env", "TMPDIR=/tmp")
	args = append(args, "--env", "HOME=/tmp")

	// Java needs JAVA_HOME
	args = append(args, "--env", "JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64")
	args = append(args, "--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/lib/jvm/java-17-openjdk-amd64/bin")

	// Logging
	logPath := workDir + "/nsjail.log"
	args = append(args, "--log", logPath)

	// Disable proc mount (security)
	args = append(args, "--disable_proc")

	// Error handling
	args = append(args, "-e")

	return args
}

// runNsjail executes an nsjail command and captures bounded stdout/stderr.
// Security fix #6: uses io.LimitedReader to cap output sizes.
func runNsjail(args []string, stdin io.Reader, maxStdout, maxStderr int) ([]byte, []byte, error) {
	if len(args) == 0 {
		return nil, nil, fmt.Errorf("no nsjail args provided")
	}

	cmd := exec.Command(args[0], args[1:]...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	// Use pipes with limited readers for bounded output capture
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting nsjail: %w", err)
	}

	// Read bounded output
	stdoutBuf := readBounded(stdoutPipe, maxStdout)
	stderrBuf := readBounded(stderrPipe, maxStderr)

	waitErr := cmd.Wait()

	return stdoutBuf, stderrBuf, waitErr
}

// readBounded reads up to maxBytes from r, appending a truncation marker if exceeded.
func readBounded(r io.Reader, maxBytes int) []byte {
	// Read maxBytes + 1 to detect truncation
	buf := make([]byte, maxBytes+1)
	n, _ := io.ReadFull(r, buf)

	if n > maxBytes {
		// Truncated — return maxBytes with marker
		result := make([]byte, maxBytes)
		copy(result, buf[:maxBytes])
		truncMarker := "\n[truncated]"
		if maxBytes > len(truncMarker) {
			copy(result[maxBytes-len(truncMarker):], truncMarker)
		}
		// Drain remaining to unblock the process
		io.Copy(io.Discard, r)
		return result
	}

	return buf[:n]
}

// nsjailMemRegex matches memory usage lines in nsjail log output.
var nsjailMemRegex = regexp.MustCompile(`\[S\]\[.*\] pid=\d+ .* rss_max_kb:(\d+)`)

// parseNsjailLog reads the nsjail log file and extracts peak memory usage.
func parseNsjailLog(logPath string) (int64, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0, err
	}

	var maxMem int64
	matches := nsjailMemRegex.FindAllSubmatch(data, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			if mem, err := strconv.ParseInt(string(match[1]), 10, 64); err == nil {
				if mem > maxMem {
					maxMem = mem
				}
			}
		}
	}

	return maxMem, nil
}

// GetNsjailVersion returns the nsjail version string.
func GetNsjailVersion(nsjailPath string) (string, error) {
	cmd := exec.Command(nsjailPath, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		// nsjail --version exits non-zero but still prints version
		output := strings.TrimSpace(out.String())
		if output != "" {
			return extractVersion(output), nil
		}
		return "", fmt.Errorf("running nsjail --version: %w", err)
	}
	return extractVersion(strings.TrimSpace(out.String())), nil
}

// extractVersion extracts a version number from nsjail output.
func extractVersion(output string) string {
	// nsjail output is typically just the version number or "NsJail v3.4"
	output = strings.TrimSpace(output)
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		// Take the first non-empty line
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				return line
			}
		}
	}
	return output
}

// RunSmokeCmd runs a language's smoke command and returns the output.
func RunSmokeCmd(smokeCmdArgs []string) (string, error) {
	if len(smokeCmdArgs) == 0 {
		return "", fmt.Errorf("no smoke command configured")
	}

	cmd := exec.Command(smokeCmdArgs[0], smokeCmdArgs[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	output := strings.TrimSpace(out.String())
	if err != nil {
		if output != "" {
			// Some tools (e.g. javac -version) write to stderr
			return output, nil
		}
		return "", fmt.Errorf("smoke command failed: %w", err)
	}
	return output, nil
}
