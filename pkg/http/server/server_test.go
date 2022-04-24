package server_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/http/server"
	"github.com/sergeii/swat4master/pkg/slice"
)

func TestHTTPServerListenAndServe(t *testing.T) {
	ready := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svr, err := server.New(
		"localhost:0", // 0 - listen an any available port
		server.WithHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			rw.WriteHeader(http.StatusTeapot)
			rw.Write(slice.Reverse(body)) // nolint:errcheck
		})),
		server.WithReadySignal(func() {
			ready <- struct{}{}
		}),
	)
	defer svr.Stop() // nolint: errcheck
	require.NoError(t, err)

	go func() {
		svr.ListenAndServe(ctx) // nolint: errcheck
	}()
	// wait for the server to start
	<-ready

	svrAddr := fmt.Sprintf("http://%s", svr.ListenAddr())
	resp, err := http.Post(svrAddr, "application/octet-stream", strings.NewReader("Hello World!")) // nolint: gosec
	require.NoError(t, err)
	assert.Equal(t, 418, resp.StatusCode)
	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "!dlroW olleH", string(respBody))
}
