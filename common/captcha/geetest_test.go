package captcha

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGenerateSignToken(t *testing.T) {
	got := GenerateSignToken("testkey", "lot123")
	want := "9809573aa3d45022b697719b8d2457daeeb0515e350a8982a54f0d04167d6f82"
	if got != want {
		t.Fatalf("sign token mismatch: got=%s want=%s", got, want)
	}
}

func TestGeetestValidateSuccess(t *testing.T) {
	client := NewGeetestClient(GeetestConfig{
		Enabled:    true,
		FailOpen:   true,
		CaptchaID:  "captcha-id",
		CaptchaKey: "captcha-key",
		APIServer:  "http://geetest.test",
		Timeout:    2000,
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := io.NopCloser(strings.NewReader(`{"result":"success","reason":""}`))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
				Header:     make(http.Header),
			}, nil
		}),
	}

	res := client.ValidateGeetestDetailed(context.Background(), "lot", "out", "pass", "time")
	if !res.Success || res.FailOpen {
		t.Fatalf("expected success without fail-open, got=%+v", res)
	}
}

func TestGeetestValidateFailure(t *testing.T) {
	client := NewGeetestClient(GeetestConfig{
		Enabled:    true,
		FailOpen:   true,
		CaptchaID:  "captcha-id",
		CaptchaKey: "captcha-key",
		APIServer:  "http://geetest.test",
		Timeout:    2000,
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := io.NopCloser(strings.NewReader(`{"result":"fail","reason":"invalid"}`))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
				Header:     make(http.Header),
			}, nil
		}),
	}

	res := client.ValidateGeetestDetailed(context.Background(), "lot", "out", "pass", "time")
	if res.Success {
		t.Fatalf("expected failure, got=%+v", res)
	}
	if res.Reason != "invalid" {
		t.Fatalf("expected reason=invalid, got=%s", res.Reason)
	}
}

func TestGeetestFailOpenOnError(t *testing.T) {
	client := NewGeetestClient(GeetestConfig{
		Enabled:    true,
		FailOpen:   true,
		CaptchaID:  "captcha-id",
		CaptchaKey: "captcha-key",
		APIServer:  "http://geetest.test",
		Timeout:    100,
	})
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		}),
	}

	res := client.ValidateGeetestDetailed(context.Background(), "lot", "out", "pass", "time")
	if !res.Success || !res.FailOpen {
		t.Fatalf("expected fail-open success, got=%+v", res)
	}
}
