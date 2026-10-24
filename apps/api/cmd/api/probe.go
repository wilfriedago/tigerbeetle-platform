package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// probe performs one GET and succeeds only on a 2xx status. The container image has no shell or
// curl, so the binary doubles as its own healthcheck: `api -probe http://127.0.0.1:8080/readyz`.
func probe(ctx context.Context, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)

	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("probe: %s answered %s", url, resp.Status)
	}

	return nil
}
