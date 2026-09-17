package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mattwalters/bubble/internal/state"
)

// ReadLogs returns the current content of a process log.
func ReadLogs(dir, name string) (string, error) {
	logPath := filepath.Join(state.WorktreeLogsDir(dir), fmt.Sprintf("%s.log", name))
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("reading log %s: %w", logPath, err)
	}
	return string(data), nil
}

// FollowLogs streams logs to w until ctx is done.
func FollowLogs(ctx context.Context, dir, name string, w io.Writer) error {
	logPath := filepath.Join(state.WorktreeLogsDir(dir), fmt.Sprintf("%s.log", name))
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("opening log %s: %w", logPath, err)
	}
	defer file.Close()

	// Output existing contents
	if _, err := io.Copy(w, file); err != nil {
		return err
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, err := file.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return writeErr
				}
			}
			if err == io.EOF {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			if err != nil {
				return err
			}
		}
	}
}
