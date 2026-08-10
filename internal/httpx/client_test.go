package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStd_DefaultTimeoutAndGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cli := Std(Options{})
	if cli == nil {
		t.Fatal("Std returned nil")
	}
	if cli.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %v, want 30s", cli.Timeout)
	}

	res, err := cli.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", res.StatusCode)
	}
}

func TestStd_UserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cli := Std(Options{
		Timeout:   5 * time.Second,
		UserAgent: "LRSS-httpx-test/1",
	})
	res, err := cli.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	res.Body.Close()
	if gotUA != "LRSS-httpx-test/1" {
		t.Fatalf("User-Agent = %q", gotUA)
	}
}
