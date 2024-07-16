package runner_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nfiacco/loadtester/internal/runner"
)

func TestQPS(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	defer server.Close()
	r := runner.NewRunner(server.URL, runner.LoadTestArgs{
		Duration: 1 * time.Second,
		Workers:  1,
		Qps:      100,
	})
	var hits uint64
	for range r.StartTest() {
		hits++
	}
	if got, want := hits, uint64(100); got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestDuration(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	defer server.Close()

	r := runner.NewRunner(server.URL, runner.LoadTestArgs{
		Duration: 1 * time.Second,
		Workers:  1,
		Qps:      100,
	})

	start := time.Now()
	for range r.StartTest() {
	}
	elapsed := time.Since(start)

	if elapsed.Round(time.Second) != time.Second {
		t.Fatalf("got: %v, want: %v", elapsed.Round(time.Second), time.Second)
	}
}
