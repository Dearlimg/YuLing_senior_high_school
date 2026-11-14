package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageviewEndpointAndBoardAuth(t *testing.T) {
	token := strings.Repeat("a", 32)
	router := testEngineWithAdmin(token)

	// 看板 API：无凭证 401，错误密码 401，正确密码 200。
	for _, header := range []string{"", "Bearer wrong-wrong-wrong-wrong-wrong-wrong"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/visitors", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("看板鉴权失败: header=%q code=%d body=%s", header, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/visitors", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"visits"`) {
		t.Fatalf("看板正常访问失败: %d %s", w.Code, w.Body.String())
	}

	// pageview 上报：合法 200，非法路径 400。
	ok := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/track/pageview",
		strings.NewReader(`{"event_key":"abc123","visitor_id":"browser-1","path":"/directory","source":"direct"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("合法上报失败: %d %s", ok.Code, ok.Body.String())
	}
	bad := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/track/pageview",
		strings.NewReader(`{"event_key":"abc123","visitor_id":"browser-1","path":"/x?phone=138","source":"direct"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(bad, req)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("带 query 的路径应被拒绝: %d %s", bad.Code, bad.Body.String())
	}

	// 隐藏看板页：路由可刷新恢复，且正常页面浏览语义不返回 404。
	page := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ops-board", nil)
	router.ServeHTTP(page, req)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<html") {
		t.Fatalf("看板页面不可访问: %d", page.Code)
	}
}
