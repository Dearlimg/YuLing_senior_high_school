package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/conf"
	"YuLing_senior_high_school/internal/controller"
	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/logic"
)

func testEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	store := dao.NewMemoryStore()
	return New(Deps{
		Config:    &conf.Config{App: conf.App{Env: "test", DemoMode: true}},
		Health:    controller.NewHealthController(nil, nil),
		Alumni:    controller.NewAlumniController(logic.NewAlumniLogic(store)),
		Community: controller.NewCommunityController(logic.NewCommunityLogic(store)),
		Limiter:   store,
	})
}

func TestEmbeddedPagesAndAPIBoundary(t *testing.T) {
	router := testEngine()
	for _, path := range []string{"/", "/directory", "/register", "/universities", "/moments", "/forum", "/privacy", "/alumni/1"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "<html lang=\"zh-CN\">") {
			t.Fatalf("page %s: %d", path, response.Code)
		}
	}
	for _, path := range []string{"/api/v1/unknown", "/assets/missing.js", "/missing"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 404 || strings.Contains(response.Body.String(), "<!doctype html>") {
			t.Fatalf("missing route returned SPA: %s", path)
		}
	}
	page := httptest.NewRecorder()
	router.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	modulePath := regexp.MustCompile(`/assets/[a-f0-9]{12}/app.js`).FindString(page.Body.String())
	if modulePath == "" {
		t.Fatal("page must use a content-versioned entry module")
	}
	module := httptest.NewRecorder()
	router.ServeHTTP(module, httptest.NewRequest(http.MethodGet, modulePath, nil))
	if module.Code != 200 || !strings.Contains(module.Body.String(), "./api.js") {
		t.Fatal("versioned module or its relative imports cannot be served")
	}
	if !strings.Contains(module.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("versioned module cache missing")
	}
}

func TestHTTPRegistrationPrivacyAndUnlockRateLimit(t *testing.T) {
	router := testEngine()
	response := httptest.NewRecorder()
	body := `{"name":"接口校友","graduation_year":2018,"phone":"13800138000","wechat":"secret-wechat","email":"private@example.com"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/alumni", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var created struct {
		Data logic.CreateResult `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/alumni", fmt.Sprintf("/api/v1/alumni/%d", created.Data.ID)} {
		public := httptest.NewRecorder()
		router.ServeHTTP(public, httptest.NewRequest(http.MethodGet, path, nil))
		for _, forbidden := range []string{"13800138000", "secret-wechat", "private@example.com", created.Data.EditCode} {
			if strings.Contains(public.Body.String(), forbidden) {
				t.Fatal("public HTTP response leaked private field")
			}
		}
		if public.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private API must not be cached")
		}
	}
	for i := 0; i < 11; i++ {
		result := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/alumni/%d/unlock", created.Data.ID), bytes.NewBufferString(`{"edit_code":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		router.ServeHTTP(result, req)
		if i == 10 && result.Code != 429 {
			t.Fatalf("untrusted forwarding headers bypassed rate limit: %d", result.Code)
		}
	}
}

func TestCrossOriginAndUnconfiguredAdminRejected(t *testing.T) {
	router := testEngine()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/alumni", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil))
	if response.Code != 503 {
		t.Fatal("unconfigured admin must be disabled")
	}
}
