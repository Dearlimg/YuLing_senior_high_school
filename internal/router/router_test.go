package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
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
	auth := logic.NewAuthLogic(store, store)
	return New(Deps{
		Config:    &conf.Config{App: conf.App{Env: "test", DemoMode: true}},
		Health:    controller.NewHealthController(nil, nil),
		Alumni:    controller.NewAlumniController(logic.NewAlumniLogic(store)),
		Community: controller.NewCommunityController(logic.NewCommunityLogic(store)),
		Limiter:   store,
		Auth:      controller.NewAuthController(auth), AuthLogic: auth,
	})
}

func TestOversizedUploadCannotBypassBrowserLimit(t *testing.T) {
	router := testEngine()
	registered := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"account":"image_user","password":"password123","name":"图片校友","graduation_year":2018}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registered, req)
	var result struct {
		Data logic.AuthResult `json:"data"`
	}
	if err := json.Unmarshal(registered.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{logic.MaxUploadImageBytes + 1, logic.MaxUploadImageBytes + (256 << 10)} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		file, err := form.CreateFormFile("image", "large.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(make([]byte, size)); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		upload := httptest.NewRequest(http.MethodPost, "/api/v1/photos", &body)
		upload.Header.Set("Content-Type", form.FormDataContentType())
		upload.Header.Set("Authorization", "Bearer "+result.Data.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, upload)
		if response.Code != 413 || !strings.Contains(response.Body.String(), "瘦身") {
			t.Fatalf("oversized upload: %d %s", response.Code, response.Body.String())
		}
	}
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
	body := `{"account":"http_test","password":"password123","name":"接口校友","graduation_year":2018,"phone":"13800138000","wechat":"secret-wechat","email":"private@example.com"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/alumni", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var created struct {
		Data logic.AuthResult `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/alumni", fmt.Sprintf("/api/v1/alumni/%d", created.Data.ID)} {
		public := httptest.NewRecorder()
		router.ServeHTTP(public, httptest.NewRequest(http.MethodGet, path, nil))
		for _, forbidden := range []string{"13800138000", "secret-wechat", "private@example.com", created.Data.Token} {
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

func TestProtectedRoutesAndOwnerProfile(t *testing.T) {
	router := testEngine()
	for _, path := range []string{"/api/v1/posts", "/api/v1/photos", "/api/v1/posts/1/replies"} {
		result := httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if result.Code != 401 {
			t.Fatalf("anonymous write %s: %d", path, result.Code)
		}
	}
	result := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"account":"owner_user","password":"password123","name":"本人","graduation_year":2018,"phone":"13800138000"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(result, req)
	if result.Code != 200 {
		t.Fatal(result.Body.String())
	}
	var body struct {
		Data logic.AuthResult `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	call := func(method, path, payload string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(payload))
		r.Header.Set("Authorization", "Bearer "+body.Data.Token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	me := call(http.MethodGet, "/api/v1/me", "")
	if me.Code != 200 || !strings.Contains(me.Body.String(), "13800138000") {
		t.Fatal("owner profile missing")
	}
	changed := call(http.MethodPut, "/api/v1/me/alumni", `{"id":999,"name":"更新本人","graduation_year":2018}`)
	if changed.Code != 200 || !strings.Contains(changed.Body.String(), fmt.Sprintf(`"id":%d`, body.Data.ID)) {
		t.Fatal("owner endpoint accepted forged ID")
	}
	legacy := call(http.MethodPut, fmt.Sprintf("/api/v1/alumni/%d", body.Data.ID), `{"name":"越权","graduation_year":2018,"edit_code":""}`)
	if legacy.Code != 403 {
		t.Fatal("account card editable without owner authentication")
	}
	if call(http.MethodPost, "/api/v1/auth/logout", "{}").Code != 200 {
		t.Fatal("logout failed")
	}
	if call(http.MethodGet, "/api/v1/me", "").Code != 401 {
		t.Fatal("logout did not revoke token")
	}
}
