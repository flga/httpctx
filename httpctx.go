// httpctx provides a context aware implementation of ListenAndServe, ListenAndServeTLS, Serve and ServeTLS, useful for integration with golang.org/x/sync/errgroup.
package httpctx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type config struct {
	beforeShutdown  func(timeout time.Duration)
	afterShutdown   func(error)
	shutdownTimeout time.Duration
}

type Option func(*config)

// WithShutdownTimeout controls the timeout for the shutdown of the http server.
// Default is 30s, a zero duration disables the timeout behavior.
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *config) {
		o.shutdownTimeout = d
	}
}

// BeforeShutdown registers fn to be called before [http.Server.Shutdown].
func BeforeShutdown(fn func(timeout time.Duration)) Option {
	return func(o *config) {
		o.beforeShutdown = fn
	}
}

// AfterShutdown registers fn to be called after [http.Server.Shutdown], err will be nil
// if [http.Server.Shutdown] didn't return an error.
func AfterShutdown(fn func(err error)) Option {
	return func(o *config) {
		o.afterShutdown = fn
	}
}

// ListenAndServe is like [http.Server.ListenAndServe] but also takes in a context.
// When the context is cancelled ListenAndServe waits until the server is shutdown, forwarding the error.
func ListenAndServe(ctx context.Context, srv *http.Server, opts ...Option) error {
	cfg := newConfig(opts...)
	return start(ctx, cfg, srv, srv.ListenAndServe)
}

// ListenAndServeTLS is like [http.Server.ListenAndServeTLS] but also takes in a context.
// When the context is cancelled ListenAndServeTLS waits until the server is shutdown, forwarding the error.
func ListenAndServeTLS(ctx context.Context, srv *http.Server, certFile, keyFile string, opts ...Option) error {
	cfg := newConfig(opts...)
	return start(ctx, cfg, srv, func() error { return srv.ListenAndServeTLS(certFile, keyFile) })
}

// Serve is like [http.Server.Serve] but also takes in a context.
// When the context is cancelled Serve waits until the server is shutdown, forwarding the error.
func Serve(ctx context.Context, srv *http.Server, ln net.Listener, opts ...Option) error {
	cfg := newConfig(opts...)
	return start(ctx, cfg, srv, func() error { return srv.Serve(ln) })
}

// ServeTLS is like [http.Server.ServeTLS] but also takes in a context.
// When the context is cancelled ServeTLS waits until the server is shutdown, forwarding the error.
func ServeTLS(ctx context.Context, srv *http.Server, ln net.Listener, certFile, keyFile string, opts ...Option) error {
	cfg := newConfig(opts...)
	return start(ctx, cfg, srv, func() error { return srv.ServeTLS(ln, certFile, keyFile) })
}

func start(ctx context.Context, cfg config, srv *http.Server, runFunc func() error) error {
	errc := make(chan error)

	go func() {
		<-ctx.Done()

		timeout := context.Background()
		if cfg.shutdownTimeout > 0 {
			var cancel context.CancelFunc
			timeout, cancel = context.WithTimeout(context.Background(), cfg.shutdownTimeout)
			defer cancel()
		}

		cfg.beforeShutdown(cfg.shutdownTimeout)
		errc <- srv.Shutdown(timeout)
	}()

	if err := runFunc(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err := <-errc
	cfg.afterShutdown(err)
	return err
}

func newConfig(opts ...Option) config {
	cfg := config{
		beforeShutdown:  func(time.Duration) {},
		afterShutdown:   func(error) {},
		shutdownTimeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}
