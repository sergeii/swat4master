package testutils

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
)

type Response struct {
	StatusCode int
	Body       string
}

type TestRequestOpt func(*http.Request, *http.Response)

func MustBindJSON(v interface{}) TestRequestOpt {
	return func(req *http.Request, resp *http.Response) {
		if resp != nil {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			if err := json.Unmarshal(body, v); err != nil {
				panic(err)
			}
		}
	}
}

func MustHaveNoBody() TestRequestOpt {
	return func(req *http.Request, resp *http.Response) {
		if resp != nil {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			if len(body) > 0 {
				panic("must have no body")
			}
		}
	}
}

func DoTestRequest(
	ts *httptest.Server, method, path string, body io.Reader, opts ...TestRequestOpt,
) Response {
	req, err := http.NewRequest(method, ts.URL+path, body) // nolint: noctx
	if err != nil {
		panic(err)
	}
	// run options that operate upon request
	for _, opt := range opts {
		opt(req, nil)
	}

	// disable redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// run options that operate upon response
	for _, opt := range opts {
		opt(nil, resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return Response{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}
}
