package main

import (
	"fmt"
	"net/http"
)

func renderMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Header().Set("Cache-Control", "no-store")

	fmt.Fprintln(w, "# HELP queue_in_course_size Number of in-flight requests tracked by the queue middleware.")
	fmt.Fprintln(w, "# TYPE queue_in_course_size gauge")
	fmt.Fprintf(w, "queue_in_course_size %d\n", inCourse.Size())
}
