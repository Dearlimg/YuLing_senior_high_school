package router

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestTracingAndPanicRecoveryDoNotLeakSecrets(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	router := testEngine()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"account":"logging_user","password":"private-password-value","name":"测试","graduation_year":2018,"phone":"13800138000"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "qa-trace-123")
	req.Header.Set("Authorization", "Bearer private-header-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get("X-Request-ID") != "qa-trace-123" {
		t.Fatal("request trace missing")
	}
	var response struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	panicRouter := gin.New()
	panicRouter.Use(requestLogging(), recoveryLogging())
	panicRouter.GET("/panic/:id", func(c *gin.Context) { panic("private-panic-value") })
	panicRequest := httptest.NewRequest(http.MethodGet, "/panic/private-path-value?keyword=private-query-value", nil)
	panicRequest.Header.Set("X-Request-ID", "invalid trace id!")
	w = httptest.NewRecorder()
	panicRouter.ServeHTTP(w, panicRequest)
	if w.Code != 500 || !validRequestID(w.Header().Get("X-Request-ID")) {
		t.Fatal("panic recovery or request ID validation failed")
	}
	output := logs.String()
	for _, secret := range []string{"private-password-value", "13800138000", "private-header-token", "private-panic-value", "private-path-value", "private-query-value", response.Data.Token} {
		if secret == "" || strings.Contains(output, secret) {
			t.Fatalf("logs leaked sensitive value %q", secret)
		}
	}
	for _, field := range []string{`"request_id":"qa-trace-123"`, `"action":"account.register"`, `"event":"panic"`, `"status":500`, `"duration_ms":`, `"business_code":0`} {
		if !strings.Contains(output, field) {
			t.Fatalf("missing log field: %s", field)
		}
	}
}
