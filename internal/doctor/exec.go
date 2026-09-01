// exec.go —— 真实命令执行（ExecRunner 的实现体）。
package doctor

import (
	"context"
	"os/exec"
	"strings"
)

// runCommand 执行命令并返回 stdout；失败时错误消息附 stderr 首行。
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			if i := strings.Index(s, "\n"); i >= 0 {
				s = s[:i]
			}
			return "", &commandError{err: err, stderr: s}
		}
		return "", err
	}
	return string(out), nil
}

// commandError 携带 stderr 摘要的错误。
type commandError struct {
	err    error
	stderr string
}

func (e *commandError) Error() string {
	return e.err.Error() + ": " + e.stderr
}

func (e *commandError) Unwrap() error { return e.err }
