package images

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
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

func TestProviderDoReadsLargeSuccessfulJSON(t *testing.T) {
	largeImage := strings.Repeat("a", providerErrorBodyLimit+128)
	client := &ProviderClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"data":[{"b64_json":"`+largeImage+`"}]}`), nil
	})}}
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/v1/images/generations", strings.NewReader(`{"prompt":"test"}`))

	payload, err := client.do(req, "生图接口")

	if err != nil {
		t.Fatalf("do returned error: %v", err)
	}
	if payload.Data[0].B64JSON != largeImage {
		t.Fatalf("large b64 payload was truncated")
	}
}

func TestRequestImageRetriesEmptyData(t *testing.T) {
	stubRetryDelay(t)
	calls := 0
	client := &ProviderClient{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(http.StatusOK, `{"created":1,"data":[]}`), nil
		}
		return response(http.StatusOK, `{"data":[{"b64_json":"aW1hZ2U="}]}`), nil
	})}}
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/v1/images/generations", strings.NewReader(`{"prompt":"test"}`))

	imageBytes, suffix, err := client.requestImage(req, "生图接口", "png")

	if err != nil {
		t.Fatalf("requestImage returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if string(imageBytes) != "image" || suffix != ".png" {
		t.Fatalf("unexpected image result: %q %s", string(imageBytes), suffix)
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

func TestGenerateAndStoreSendsRequestParams(t *testing.T) {
	var payload map[string]any
	client := &ProviderClient{
		cfg: config.Config{ImageStorageDir: t.TempDir()},
		client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request payload: %v", err)
			}
			return response(http.StatusOK, `{"data":[{"b64_json":"aW1hZ2U="}]}`), nil
		})},
	}
	compression := 82

	path, err := client.GenerateAndStore("test", GenerationParams{
		Size:              "1536x1024",
		Quality:           "high",
		OutputFormat:      "webp",
		OutputCompression: &compression,
		Moderation:        "low",
	}, Provider{Name: "test", ProviderType: "openai_compatible", Model: "gpt-image-2", APIBase: "https://example.test", APIKey: "key"})

	if err != nil {
		t.Fatalf("GenerateAndStore returned error: %v", err)
	}
	if !strings.HasSuffix(path, ".webp") {
		t.Fatalf("expected webp suffix, got %s", path)
	}
	for key, expected := range map[string]any{
		"model":              "gpt-image-2",
		"prompt":             "test",
		"size":               "1536x1024",
		"quality":            "high",
		"output_format":      "webp",
		"output_compression": float64(82),
		"moderation":         "low",
	} {
		if payload[key] != expected {
			t.Fatalf("payload[%s] = %#v, want %#v; payload=%#v", key, payload[key], expected, payload)
		}
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
