package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/segmentio/fasthash/fnv1a"

	"golang.org/x/sync/semaphore"
)

type queuedReq struct {
	w http.ResponseWriter
	r *http.Request
}

var buckets = func() [52]*semaphore.Weighted {
	var s [52]*semaphore.Weighted
	for i := range s {
		s[i] = semaphore.NewWeighted(1)
	}
	return s
}()

var (
	queueAcquireTimeoutError          = errors.New("QAT")
	redirectToCloudflareCacheHitMaybe = errors.New("RTCCHM")
	requestCanceledAbortEverything    = errors.New("RCAE")
	serverUnderHeavyLoad              = errors.New("SUHL")
	queueAcquireTimeout               = 6 * time.Second
)

var inCourse = xsync.NewMapOfWithHasher[uint64, struct{}](
	func(key uint64, seed uint64) uint64 { return key },
)

func await(ctx context.Context) {
	val := ctx.Value("ticket")
	if val == nil {
		return
	}
	code := val.(int)

	reqNum := ctx.Value("reqNum").(uint64)
	if _, ok := inCourse.LoadOrStore(reqNum, struct{}{}); ok {
		// we've already acquired a semaphore for this request, no need to do it again
		return
	}

	sem := buckets[code]
	if sem.TryAcquire(1) {
		// means we're the first to use this bucket
		go func() {
			// we'll release it after the request is answered
			<-ctx.Done()
			sem.Release(1)
		}()
	} else {
		// otherwise someone else has already locked it, so we wait
		acquireTimeout, cancel := context.WithTimeoutCause(ctx, queueAcquireTimeout, queueAcquireTimeoutError)
		defer cancel()

		err := sem.Acquire(acquireTimeout, 1)
		if err == nil {
			// got it soon enough
			sem.Release(1)
			panic(redirectToCloudflareCacheHitMaybe)
		} else if context.Cause(acquireTimeout) == queueAcquireTimeoutError {
			// took too long
			panic(serverUnderHeavyLoad)
		} else {
			// request was canceled
			panic(requestCanceledAbortEverything)
		}
	}
}

var reqNumSource atomic.Uint64

func queueMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" || strings.HasPrefix(r.URL.Path, "/njump/static/") {
			next.ServeHTTP(w, r)
			return
		}

		reqNum := reqNumSource.Add(1)

		// these will be used when we later call await(ctx)
		ticket := int(fnv1a.HashString64(r.URL.Path) % uint64(len(buckets)))
		ctx := context.WithValue(
			context.WithValue(
				r.Context(),
				"reqNum", reqNum,
			),
			"ticket", ticket,
		)

		defer func() {
			err := recover()

			if err == nil {
				return
			}

			switch err {
			// if we are not the first to request this we will wait for the underlying page to be loaded
			// then we will be redirect to open it again, so hopefully we will hit the cloudflare cache this time
			case redirectToCloudflareCacheHitMaybe:
				path := r.URL.Path
				if r.URL.RawQuery != "" {
					path += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, path, http.StatusFound)

			case serverUnderHeavyLoad:
				w.WriteHeader(504)
				w.Write([]byte("server under heavy load, please try again in a couple of seconds"))
				return

			case requestCanceledAbortEverything:
				return

			default:
				trace := trackError(r, err)
				w.WriteHeader(500)

				fmt.Fprintf(w, "%s\n", err)
				for _, line := range trace {
					fmt.Fprintf(w, "%s\n", line)
				}
			}
		}()

		defer func() {
			inCourse.Delete(reqNum)
		}()

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
