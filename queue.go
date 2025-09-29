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
		s[i] = semaphore.NewWeighted(2)
	}
	return s
}()

var (
	redirectToCloudflareCacheHitMaybe = errors.New("RTCCHM")
	requestCanceledAbortEverything    = errors.New("RCAE")
)

func await(ctx context.Context) {
	val := ctx.Value("code")
	if val == nil {
		return
	}

	code := val.(string)
	sem := buckets[int(fnv1a.HashString64(code)%uint64(len(buckets)))]

	acquireTimeout, cancel := context.WithTimeoutCause(ctx, time.Second*9, redirectToCloudflareCacheHitMaybe)
	defer cancel()

	if err := sem.Acquire(acquireTimeout, 1); err != nil {
		if context.Cause(acquireTimeout) == redirectToCloudflareCacheHitMaybe {
			panic(redirectToCloudflareCacheHitMaybe)
		} else {
			panic(requestCanceledAbortEverything)
		}
	}

	go func() {
		<-ctx.Done()
		sem.Release(1)
	}()
}

func queueMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "code", r.URL.Path)

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

				select {
				case <-time.After(time.Second * 9):
					http.Redirect(w, r, path, http.StatusFound)
				case <-ctx.Done():
				}

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
