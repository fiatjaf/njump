package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderMetricsReportsQueueSize(t *testing.T) {
	resetQueueState(t)

	const sampleSize = 3
	for i := 0; i < sampleSize; i++ {
		inCourse.Store(uint64(i+1), struct{}{})
	}

	req := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
	rec := httptest.NewRecorder()

	renderMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "queue_in_course_size 3") {
		t.Fatalf("expected metric output to contain size 3, got %q", body)
	}
}
