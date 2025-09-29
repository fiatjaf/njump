package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var globalErrorTrackerMutex = sync.Mutex{}
var globalErrorFile = "/tmp/njump-errors"

func trackError(r *http.Request, trackedError any) []string {
	globalErrorTrackerMutex.Lock()
	defer globalErrorTrackerMutex.Unlock()

	dir := filepath.Dir(globalErrorFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal().Err(err).Msg("failed to create error log directory")
		return nil
	}

	file, err := os.OpenFile(globalErrorFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Error().Err(err).Msg("failed to open error tracking file")
		return nil
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("when: %s\n", time.Now().Format(time.DateTime)))
	file.WriteString(fmt.Sprintf("at: %s %s\n", r.Method, r.URL.Path))
	file.WriteString(fmt.Sprintf("by whom: %s; ip: %s; from %s\n",
		r.Header.Get("user-agent"), actualIP(r), r.Header.Get("referer")))
	file.WriteString(fmt.Sprintf("what: %v\n", trackedError))

	file.WriteString(fmt.Sprintf("trace:\n"))
	trace := captureStackTrace(5 /* / skip 5 frames: recoveryMiddleware, captureStackTrace, trackError etc */)
	for _, line := range trace {
		file.WriteString(fmt.Sprintf("%s\n", line))
	}

	file.WriteString("\n---\n")

	// return the stack trace so we can display it to the user
	return trace
}

func captureStackTrace(skip int) []string {
	frames := make([]string, 0, 11)
	stack := strings.Split(string(debug.Stack()), "\n")

	frames = append(frames, stack[0])

	for i := 1 + skip*2; i < min(1+skip*2+20, len(stack)); i++ { // capture up to 10 frames
		line := stack[i]

		if strings.HasPrefix(line, "\t") {
			if idx := strings.Index(line, "/njump/"); idx != -1 {
				line = "\t" + line[idx:]
			}
		}

		frames = append(frames, line)
	}

	return frames
}
