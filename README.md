# httpctx
[![Go Reference](https://pkg.go.dev/badge/github.com/flga/httpctx.svg)](https://pkg.go.dev/github.com/flga/httpctx)
[![Go Report Card](https://goreportcard.com/badge/github.com/flga/httpctx)](https://goreportcard.com/report/github.com/flga/httpctx)

A context aware implementation of ListenAndServe, ListenAndServeTLS, Serve and ServeTLS, useful for integration with golang.org/x/sync/errgroup.

Usage:
```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/flga/httpctx"
	"golang.org/x/sync/errgroup"
)

func main() {
	g, ctx := errgroup.WithContext(context.Background())

	// setup a failure in ~5s
	g.Go(func() error {
		time.Sleep(5 * time.Second)
		log.Println("something went wrong!")
		return errors.New("boom")
	})

	// the actual server
	g.Go(func() error {
		srv := http.Server{
			Addr: ":0",
		}

		log.Println("listening")
		return httpctx.ListenAndServe(
			ctx, // when this ctx expires, the server starts the shutdown process
			&srv,
			// wait for at most 30s when shutting down
			httpctx.WithShutdownTimeout(30*time.Second),
			// called just before the shutdown process starts, with the configured timeout
			httpctx.BeforeShutdown(func(timeout time.Duration) {
				log.Printf("shutting down, waiting for at most %.0fs", timeout.Seconds())
			}),
			// called after the shutdown process ends, with the error it returned (if any)
			httpctx.AfterShutdown(func(err error) {
				if err != nil {
					// probably a context.DeadlineExceeded
					log.Printf("unclean shutdown: %s", err)
					return
				}
				log.Println("shutdown complete")
			}),
		)
	})

	if err := g.Wait(); err != nil {
		panic(err)
	}
}

```