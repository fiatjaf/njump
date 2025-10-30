package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/segmentio/fasthash/fnv1a"
	"golang.org/x/sync/semaphore"
)

func resetQueueState(t *testing.T) {
	t.Helper()

	reqNumSource.Store(0)
	inCourse = xsync.NewMapOfWithHasher[uint64, struct{}](
		func(key uint64, seed uint64) uint64 { return key },
	)
	oldErrorFile := globalErrorFile
	globalErrorFile = filepath.Join(t.TempDir(), "njump-errors")
	oldQueueTimeout := queueAcquireTimeout
	queueAcquireTimeout = 6 * time.Second
	t.Cleanup(func() {
		globalErrorFile = oldErrorFile
		queueAcquireTimeout = oldQueueTimeout
	})
}

func TestQueueMiddlewareDeletesEntryOnRedirectPanic(t *testing.T) {
	resetQueueState(t)

	const path = "/queue-test"
	ticket := int(fnv1a.HashString64(path) % uint64(len(buckets)))

	originalSem := buckets[ticket]
	sem := semaphore.NewWeighted(1)
	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("failed to prepare semaphore: %v", err)
	}
	buckets[ticket] = sem
	t.Cleanup(func() {
		buckets[ticket] = originalSem
	})

	go func() {
		time.Sleep(5 * time.Millisecond)
		sem.Release(1)
	}()

	handler := queueMiddleware(func(w http.ResponseWriter, r *http.Request) {
		await(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect status, got %d", rec.Code)
	}

	if size := inCourse.Size(); size != 0 {
		t.Fatalf("expected inCourse to be empty, got %d", size)
	}
}

func TestQueueMiddlewareDeletesEntryOnGenericPanic(t *testing.T) {
	resetQueueState(t)

	handler := queueMiddleware(func(w http.ResponseWriter, r *http.Request) {
		reqNum := r.Context().Value("reqNum").(uint64)
		inCourse.Store(reqNum, struct{}{})
		panic(errors.New("boom"))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d", rec.Code)
	}

	if size := inCourse.Size(); size != 0 {
		t.Fatalf("expected inCourse to be empty, got %d", size)
	}
}

func TestQueueMiddlewarePanicUnderLoad(t *testing.T) {
	resetQueueState(t)

	const path = "/queue-load"
	ticket := int(fnv1a.HashString64(path) % uint64(len(buckets)))

	originalSem := buckets[ticket]
	sem := semaphore.NewWeighted(1)
	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("failed to prepare semaphore: %v", err)
	}
	buckets[ticket] = sem
	t.Cleanup(func() {
		sem.Release(1)
		buckets[ticket] = originalSem
	})

	oldTimeout := queueAcquireTimeout
	queueAcquireTimeout = 5 * time.Millisecond
	t.Cleanup(func() {
		queueAcquireTimeout = oldTimeout
	})

	handler := queueMiddleware(func(w http.ResponseWriter, r *http.Request) {
		await(r.Context())
	})

	var wg sync.WaitGroup
	const workers = 64
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusGatewayTimeout {
				t.Errorf("expected 504 status, got %d", rec.Code)
			}
		}()
	}

	wg.Wait()

	if size := inCourse.Size(); size != 0 {
		t.Fatalf("expected inCourse to be empty, got %d", size)
	}
}

func BenchmarkQueueMiddlewareHappyPath(b *testing.B) {
	reqNumSource.Store(0)
	inCourse = xsync.NewMapOfWithHasher[uint64, struct{}](
		func(key uint64, seed uint64) uint64 { return key },
	)
	oldErrorFile := globalErrorFile
	globalErrorFile = filepath.Join(b.TempDir(), "njump-errors")
	defer func() {
		globalErrorFile = oldErrorFile
	}()

	handler := queueMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/bench", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rr.Code)
		}
	}
}
