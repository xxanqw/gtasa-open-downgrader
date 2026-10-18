package openmpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const fixture = `[{"core":{"ip":"203.0.113.7:7777","hn":"Test server","pc":12,"pm":100,"gm":"Freeroam","la":"English","pa":true,"vn":"omp 1.5","omp":true},"ru":{"mapname":"San Andreas","allowed_clients":"0.3.7, 0.3.DL","weburl":"example.com","unknown":"ignore"},"lastUpdated":"2026-10-13T12:00:00Z"}]`

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOfficialFullIndexSchemaAndCacheRoundtrip(t *testing.T) {
	client := New()
	client.HTTP.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != Endpoint || req.Method != http.MethodGet || req.Header.Get("Accept") != "application/json" || req.Body != nil {
			t.Fatal("unexpected API request or credential payload")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fixture)), Header: make(http.Header)}, nil
	})
	servers, err := client.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Core.Address != "203.0.113.7:7777" || !servers[0].Core.Password || servers[0].Rules["allowed_clients"] == "" || servers[0].Rules["unknown"] != "" {
		t.Fatal("full API schema lost", servers)
	}
	cached, err := FromCache(Encode(servers))
	if err != nil || len(cached) != 1 || cached[0].Core.Name != "Test server" {
		t.Fatal("cache roundtrip failed", err)
	}
}
func TestMalformedOversizedOrInvalidIndexFailsClosed(t *testing.T) {
	for _, data := range []string{"null", "{}", "[", fixture + " garbage", strings.Repeat(" ", MaxResponseBytes+1), `[{"core":{"ip":"https://evil.test:7777","pc":1,"pm":2}}]`, `[{"core":{"ip":"203.0.113.7:0"}}]`, `[{"core":{"ip":"203.0.113.7:7777","pc":-1}}]`} {
		if _, err := FromCache(data); err == nil {
			t.Fatal("invalid index accepted")
		}
	}
	tooMany := `[` + strings.Repeat(`{"core":{"ip":"203.0.113.7:7777"}},`, MaxServers) + `{"core":{"ip":"203.0.113.8:7777"}}]`
	if _, err := FromCache(tooMany); err == nil {
		t.Fatal("excessive server count accepted")
	}
	if _, err := FromCache(`[]`); err != nil {
		t.Fatal("empty index must be valid", err)
	}
}
func TestIndexDeduplicatesAndBoundsPlainText(t *testing.T) {
	data := `[{"core":{"ip":"203.0.113.7:7777","hn":"` + strings.Repeat("Ж", 300) + `\n\u0000"}},{"core":{"ip":"203.0.113.7:7777"}},{"core":{"ip":"invalid"}}]`
	servers, err := FromCache(data)
	if err != nil || len(servers) != 1 || len([]rune(servers[0].Core.Name)) != 256 {
		t.Fatal("unbounded/duplicate server data", err)
	}
}
func TestLiveOfficialServerIndex(t *testing.T) {
	if os.Getenv("OPENMP_API_LIVE") != "1" {
		t.Skip("opt-in read-only upstream API check")
	}
	servers, err := New().List(context.Background())
	if err != nil || len(servers) == 0 {
		t.Fatal("official full index could not be decoded", err)
	}
	t.Logf("decoded %d valid servers from the official full index", len(servers))
}

func TestHTTPStatusLimitsCancellationAndRedirectPolicy(t *testing.T) {
	for _, status := range []int{429, 500} {
		c := New()
		c.HTTP.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("error"))}, nil
		})
		if _, err := c.List(context.Background()); err == nil {
			t.Fatal("HTTP error accepted")
		}
	}
	large := New()
	large.HTTP.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: MaxResponseBytes + 1, Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})
	if _, err := large.List(context.Background()); err == nil {
		t.Fatal("oversized declared response accepted")
	}
	c := New()
	c.HTTP.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.List(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("request did not honor cancellation", err)
	}
	req, _ := http.NewRequest("GET", "https://evil.example/", nil)
	if c.HTTP.CheckRedirect(req, nil) == nil {
		t.Fatal("cross-host redirect accepted")
	}
}
