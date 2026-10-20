package pgbeam

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// withQuery appends params' form-tagged fields to path as a query string.
func withQuery(path string, params any) string {
	if q := encodeQuery(params); q != "" {
		return path + "?" + q
	}
	return path
}

// doRaw executes a request whose success response is not JSON (e.g. a
// text/csv export) and returns the body bytes unchanged. It goes through
// transport.do so retries and error decoding match every other call.
func doRaw(t *transport, ctx context.Context, method, path, accept string, body any) ([]byte, error) {
	client := *t.httpClient
	next := client.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	client.Transport = rawRoundTripper{next: next, accept: accept}

	rt := *t
	rt.httpClient = &client

	var out []byte
	if err := rt.do(ctx, method, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// rawRoundTripper asks for accept instead of JSON and re-encodes a success
// body as a JSON byte string, which transport.do decodes back to the exact
// bytes the server sent. Error responses pass through for APIError parsing.
type rawRoundTripper struct {
	next   http.RoundTripper
	accept string
}

func (rt rawRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Accept", rt.accept)

	resp, err := rt.next.RoundTrip(req)
	if err != nil || resp.StatusCode >= 400 {
		return resp, err
	}

	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	enc, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(enc))
	resp.ContentLength = int64(len(enc))
	resp.Header.Set("Content-Length", strconv.Itoa(len(enc)))
	return resp, nil
}
