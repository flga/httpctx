package httpctx_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/flga/httpctx"
)

func Example() {
	server := http.Server{
		Addr: ":0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start shutdown after 1s
	time.AfterFunc(time.Second, func() {
		fmt.Println("shutting down")
		cancel()
	})

	// will block until the server is shutdown
	fmt.Println("listening")
	if err := httpctx.ListenAndServe(ctx, &server); err != nil {
		panic(err)
	}

	fmt.Println("done")
	// Output: listening
	// shutting down
	// done
}
