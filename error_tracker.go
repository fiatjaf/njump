package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ErrorTracker struct {
	filePath string
	mu       sync.Mutex
}

type ErrorEntry struct {
	Timestamp   time.Time         `json:"timestamp"`
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	Error       string            `json:"error,omitempty"`
	StackTrace  []string          `json:"stack_trace,omitempty"`
	RequestInfo *RequestContext   `json:"request_info,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type RequestContext struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Referer   string `json:"referer"`
}

var globalTracker *ErrorTracker

// Initializes the global error tracker
func InitErrorTracker(filePath string) error {
	if filePath == "" {
		filePath = "/tmp/njump-errors.jsonl"
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create error log directory: %w", err)
	}

	globalTracker = &ErrorTracker{
		filePath: filePath,
	}

	log.Info().Str("path", filePath).Msg("error tracker initialized")
	return nil
}

// Logs an error to the error tracking file
func TrackError(level, message string, err error, r *http.Request, metadata map[string]string) {
	if globalTracker == nil {
		return
	}

	entry := ErrorEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
		Metadata:  metadata,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	if r != nil {
		entry.RequestInfo = &RequestContext{
			Method:    r.Method,
			Path:      r.URL.Path,
			IP:        actualIP(r),
			UserAgent: r.Header.Get("User-Agent"),
			Referer:   r.Header.Get("Referer"),
		}
	}

	// Capture stack trace for errors and panics
	if level == "error" || level == "panic" {
		entry.StackTrace = captureStackTrace(3) // Skip 3 frames: captureStackTrace, TrackError, caller
	}

	globalTracker.writeEntry(entry)
}

func (et *ErrorTracker) writeEntry(entry ErrorEntry) {
	et.mu.Lock()
	defer et.mu.Unlock()

	file, err := os.OpenFile(et.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Error().Err(err).Msg("failed to open error tracking file")
		return
	}
	defer file.Close()

	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal error entry")
		return
	}

	writer := bufio.NewWriter(file)
	writer.Write(jsonData)
	writer.WriteByte('\n')
	writer.Flush()

	// Rotate file if it gets too large (>10MB)
	if stat, err := file.Stat(); err == nil && stat.Size() > 10*1024*1024 {
		et.rotateFile()
	}
}

func (et *ErrorTracker) rotateFile() {
	oldPath := et.filePath
	newPath := et.filePath + ".old"

	// Remove old backup if exists
	os.Remove(newPath)

	// Move current file to backup
	if err := os.Rename(oldPath, newPath); err != nil {
		log.Error().Err(err).Msg("failed to rotate error log file")
	}
}

func captureStackTrace(skip int) []string {
	var frames []string

	for i := skip; i < skip+10; i++ { // Capture up to 10 frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		name := "unknown"
		if fn != nil {
			name = fn.Name()
		}

		// Clean up file path to just show relative path
		if strings.Contains(file, "njump") {
			parts := strings.Split(file, "njump")
			if len(parts) > 1 {
				file = "njump" + parts[len(parts)-1]
			}
		}

		frames = append(frames, fmt.Sprintf("%s:%d %s", file, line, name))
	}

	return frames
}

// Logs an application error, filtering out noisy network timeouts
func TrackAppError(message string, err error, r *http.Request, metadata map[string]string) {
	// Skip noisy network timeout errors that don't indicate app problems
	if err != nil {
		errStr := strings.ToLower(err.Error())
		timeoutKeywords := []string{
			"context deadline exceeded",
			"context canceled",
			"i/o timeout",
			"connection reset by peer",
			"broken pipe",
			"client disconnected",
			"http2: stream closed",
			"connection refused",
			"no such host",
			"network is unreachable",
		}

		for _, keyword := range timeoutKeywords {
			if strings.Contains(errStr, keyword) {
				return
			}
		}
	}

	TrackError("error", message, err, r, metadata)
}

func TrackGenericError(message string, err error) {
	TrackAppError(message, err, nil, nil)
}
