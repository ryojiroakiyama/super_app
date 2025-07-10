package googleauth

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"time"
)

// loggingTransport wraps an existing http.RoundTripper and logs
// outgoing Google API requests and the corresponding responses.
// It is lightweight and only records method, URL, latency and status.
// For POST / PUT requests the body is also dumped (as string) for easier debugging.
// NOTE: Do not enable in production if request bodies may contain sensitive data.
// This is intended for local debugging.
//
// To enable it, GmailServiceFromToken wraps the underlying transport automatically.
// If you want to disable logging, comment out the wrapper section in that function.
//
// Inspired by https://github.com/googleapis/google-api-go-client/issues/101 and similar snippets.

type loggingTransport struct {
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log basic request information
	log.Printf("[gmail] -> %s %s", req.Method, req.URL.String())

	// For mutating requests, attempt to read and log body (non-destructive)
	if req.Body != nil && (req.Method == http.MethodPost || req.Method == http.MethodPut) {
		var buf bytes.Buffer
		tee := io.TeeReader(req.Body, &buf)
		bodyBytes, _ := io.ReadAll(tee)
		log.Printf("[gmail] request body: %s", string(bodyBytes))
		// restore body so underlying transport can read it
		req.Body = io.NopCloser(&buf)
	}

	// Delegate to underlying transport
	rt := t.base
	if rt == nil {
		rt = http.DefaultTransport
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		log.Printf("[gmail] <- error: %v (elapsed %v)", err, time.Since(start))
		return resp, err
	}

	log.Printf("[gmail] <- status: %d (elapsed %v)", resp.StatusCode, time.Since(start))
	return resp, err
}
