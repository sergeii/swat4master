package httpserver_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/http/httpserver"
)

func TestHTTPServerListenAndServe(t *testing.T) {
	ready := make(chan struct{})
	svr, err := httpserver.New(
		"localhost:0", // 0 - listen an any available port
		httpserver.WithHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			rw.WriteHeader(http.StatusTeapot)
			resp := body
			slices.Reverse(resp)
			rw.Write(resp) // nolint:errcheck
		})),
		httpserver.WithReadySignal(func(net.Addr) {
			close(ready)
		}),
	)
	defer svr.Stop(context.TODO()) // nolint: errcheck
	require.NoError(t, err)

	go func() {
		svr.ListenAndServe() // nolint: errcheck
	}()
	// wait for the server to start
	<-ready

	svrAddr := fmt.Sprintf("http://%s", svr.ListenAddr())
	resp, err := http.Post(svrAddr, "application/octet-stream", strings.NewReader("Hello World!")) // nolint: gosec
	require.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()
	assert.Equal(t, 418, resp.StatusCode)
	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "!dlroW olleH", string(respBody))
}
