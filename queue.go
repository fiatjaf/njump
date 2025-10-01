package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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
)

func await(ctx context.Context) {
	val := ctx.Value("code")
	if val == nil {
		return
	}
	code := val.(int)

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
		acquireTimeout, cancel := context.WithTimeoutCause(ctx, time.Second*6, queueAcquireTimeoutError)
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

func queueMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := int(fnv1a.HashString64(r.URL.Path) % uint64(len(buckets)))
		ctx := context.WithValue(r.Context(), "code", code)

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

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
