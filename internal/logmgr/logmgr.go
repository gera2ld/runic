package logmgr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeFilename(s string) string {
	return safeFilenameRe.ReplaceAllString(s, "_")
}

type StreamLogger struct {
	file *os.File
	mw   io.Writer
}

func NewStreamLogger(logDir, actionID string) (*StreamLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("task_%s_%s.log", sanitizeFilename(actionID), ts)
	path := filepath.Join(logDir, filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)
	return &StreamLogger{file: f, mw: mw}, nil
}

func (s *StreamLogger) Writer() io.Writer {
	return s.mw
}

func (s *StreamLogger) FilePath() string {
	return s.file.Name()
}

func (s *StreamLogger) Close() error {
	return s.file.Close()
}

type DBCleaner interface {
	DeleteHistoryBefore(t time.Time) (int64, error)
}

func Clean(logDir string, dbc DBCleaner, days int, maxNum int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	deleted, err := dbc.DeleteHistoryBefore(cutoff)
	if err != nil {
		return err
	}
	fmt.Printf("[clean] deleted %d history records older than %d days\n", deleted, days)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type logFile struct {
		name    string
		modTime time.Time
		path    string
	}

	var logs []logFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		logs = append(logs, logFile{
			name:    e.Name(),
			modTime: info.ModTime(),
			path:    filepath.Join(logDir, e.Name()),
		})
	}

	var deletedFiles int
	for _, lf := range logs {
		if lf.modTime.Before(cutoff) {
			if err := os.Remove(lf.path); err == nil {
				deletedFiles++
			}
		}
	}
	fmt.Printf("[clean] deleted %d log files older than %d days\n", deletedFiles, days)

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].modTime.Before(logs[j].modTime)
	})

	remaining := len(logs) - deletedFiles
	if remaining > maxNum {
		toRemove := remaining - maxNum
		removed := 0
		for i := 0; i < len(logs) && removed < toRemove; i++ {
			if _, err := os.Stat(logs[i].path); os.IsNotExist(err) {
				continue
			}
			if err := os.Remove(logs[i].path); err == nil {
				removed++
			}
		}
		fmt.Printf("[clean] deleted %d excess log files (max=%d)\n", removed, maxNum)
	}

	return nil
}
