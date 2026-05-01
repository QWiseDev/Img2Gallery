package images

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProviderDoRetriesRetryableStatus(t *testing.T) {
	stubRetryDelay(t)
	calls := 0
	client := &ProviderClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(req.Body)
		if string(body) != `{"prompt":"test"}` {
			t.Fatalf("request body was not reset, got %q", string(body))
		}
		if calls == 1 {
			return response(http.StatusBadGateway, "bad gateway"), nil
		}
		return response(http.StatusOK, `{"data":[{"b64_json":"aW1hZ2U="}]}`), nil
	})}}
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/v1/images/generations", strings.NewReader(`{"prompt":"test"}`))

	payload, err := client.do(req, "生图接口")

	if err != nil {
		t.Fatalf("do returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if payload.Data[0].B64JSON != "aW1hZ2U=" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestProviderDoRetriesNetworkTimeout(t *testing.T) {
	stubRetryDelay(t)
	calls := 0
	client := &ProviderClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls < providerAttempts {
			return nil, &net.DNSError{Err: "i/o timeout", IsTimeout: true}
		}
		return response(http.StatusOK, `{"data":[{"url":"https://example.test/image.png"}]}`), nil
	})}}
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/v1/images/generations", strings.NewReader(`{"prompt":"test"}`))

	_, err := client.do(req, "生图接口")

	if err != nil {
		t.Fatalf("do returned error: %v", err)
	}
	if calls != providerAttempts {
		t.Fatalf("expected %d calls, got %d", providerAttempts, calls)
	}
}

func TestProviderDoDoesNotRetryBadRequest(t *testing.T) {
	stubRetryDelay(t)
	calls := 0
	client := &ProviderClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return response(http.StatusBadRequest, `{"error":"bad prompt"}`), nil
	})}}
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/v1/images/generations", strings.NewReader(`{"prompt":"test"}`))

	_, err := client.do(req, "生图接口")

	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected 400 error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func stubRetryDelay(t *testing.T) {
	t.Helper()
	original := providerRetryDelay
	providerRetryDelay = func(int) time.Duration {
		return 0
	}
	t.Cleanup(func() { providerRetryDelay = original })
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status) + " " + http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
