package httpctx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestUnblocksIfRunFuncFails(t *testing.T) {
	t.Parallel()

	// eat up a random port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := http.Server{
		Addr: ln.Addr().String(), // used port
	}
	if err := ListenAndServe(context.Background(), &srv); err == nil {
		t.Fatal("should have failed")
	}
}

func TestBlockUntilCtxExpires(t *testing.T) {
	t.Parallel()

	test := func(name string, listenFunc func(context.Context, *http.Server) error) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := &http.Server{
				Addr: ":0",
			}

			// kill the server after 1s
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			errc := make(chan error)
			go func() {
				errc <- listenFunc(ctx, srv)
			}()

			select {
			case err := <-errc:
				if err != nil {
					t.Fatal(err)
				}

			case <-time.After(2 * time.Second):
				t.Fatal("shutdown timeout")
			}
		})
	}

	test("ListenAndServe", func(ctx context.Context, s *http.Server) error {
		return ListenAndServe(ctx, s)
	})
	test("ListenAndServeTLS", func(ctx context.Context, s *http.Server) error {
		return ListenAndServeTLS(ctx, s, "testdata/cert.pem", "testdata/key.pem")
	})
	test("Serve", func(ctx context.Context, s *http.Server) error {
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		return Serve(ctx, s, ln)
	})
	test("ServeTLS", func(ctx context.Context, s *http.Server) error {
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		return ServeTLS(ctx, s, ln, "testdata/cert.pem", "testdata/key.pem")
	})
}

func TestShutdownTimeout(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	s := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			// unblock the client
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// but block the handler
			time.Sleep(30 * time.Second)
		}),
	}
	errc := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the server, has 1s to shutdown
	go func() {
		errc <- Serve(ctx, &s, ln, WithShutdownTimeout(time.Second))
	}()

	// wait for server to start (unfortunate) and kick off the handler
	time.Sleep(time.Second)
	if _, err := http.Get("http://" + addr); err != nil {
		t.Fatal(err)
	}

	// kill it, should timeout since handler is blocked
	cancel()
	if err := <-errc; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestShutdownNoTimeout(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	s := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			// unblock the client
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// but block the handler
			time.Sleep(1 * time.Second)
		}),
	}
	errc := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start the server, has no timeout
	go func() {
		errc <- Serve(ctx, &s, ln, WithShutdownTimeout(0))
	}()

	// wait for server to start (unfortunate) and kick off the handler
	time.Sleep(time.Second)
	if _, err := http.Get("http://" + addr); err != nil {
		t.Fatal(err)
	}

	// kill it, should NOT timeout
	cancel()
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}

func TestHooks(t *testing.T) {
	t.Parallel()

	srv := http.Server{
		Addr: ":0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beforeShutdown := make(chan struct{})
	afterShutdown := make(chan struct{})

	errc := make(chan error)
	go func() {
		errc <- ListenAndServe(
			ctx,
			&srv,
			BeforeShutdown(func(timeout time.Duration) { close(beforeShutdown) }),
			AfterShutdown(func(err error) { close(afterShutdown) }),
		)
	}()
	cancel()

	select {
	case <-beforeShutdown:
	case <-time.After(time.Second):
		t.Fatal("before shutdown not called")
	}

	select {
	case <-afterShutdown:
	case <-time.After(time.Second):
		t.Fatal("after shutdown not called")
	}
}
