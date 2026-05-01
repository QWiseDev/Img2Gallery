package images

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const providerAttempts = 3

const (
	providerDialTimeout    = 60 * time.Second
	providerErrorBodyLimit = 1 << 20
)

var providerRetryDelay = func(attempt int) time.Duration {
	return time.Duration(attempt-1) * time.Second
}

func newProviderHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: providerDialTimeout, KeepAlive: 30 * time.Second}
	transport.DialContext = dialer.DialContext
	transport.TLSHandshakeTimeout = providerDialTimeout
	transport.ResponseHeaderTimeout = requestTimeout
	return &http.Client{Timeout: requestTimeout, Transport: transport}
}

func shouldRetryProviderError(err error, attempt int) bool {
	if attempt >= providerAttempts {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		strings.Contains(strings.ToLower(err.Error()), "connection reset")
}

func shouldRetryProviderStatus(statusCode, attempt int) bool {
	if attempt >= providerAttempts {
		return false
	}
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func resetRequestBody(req *http.Request) error {
	if req.Body == nil {
		return nil
	}
	if req.GetBody == nil {
		return errors.New("生图接口请求体无法重试")
	}
	body, err := req.GetBody()
	if err != nil {
		return errors.New("生图接口请求体重置失败：" + err.Error())
	}
	req.Body = body
	return nil
}

func waitProviderRetry(req *http.Request, attempt int) {
	delay := providerRetryDelay(attempt)
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-req.Context().Done():
	case <-timer.C:
	}
}

func providerStatusError(label, status string, raw []byte) error {
	detail := strings.TrimSpace(strings.ReplaceAll(string(raw), "\n", " "))
	if len(detail) > 240 {
		detail = detail[:240]
	}
	if detail != "" {
		return errors.New(label + "返回 " + status + "：" + detail)
	}
	return errors.New(label + "返回 " + status)
}
