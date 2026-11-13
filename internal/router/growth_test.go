package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"YuLing_senior_high_school/internal/logic"
)

func TestPublicClaimAndAdminMetricsBoundary(t *testing.T) {
	token := "test-growth-admin-token-123456789012345"
	router := testEngineWithAdmin(token)
	request := func(method, path, body, auth string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	result := request(
		"POST",
		"/api/v1/alumni/claim",
		`{"name":"免账号校友","graduation_year":2018,"class_name":"高三1班",`+
			`"attribution":{"visitor_id":"test_browser","source":"class_invite",`+
			`"campaign":"reunion","share_id":"test_share"}}`,
		"",
	)
	if result.Code != 200 {
		t.Fatalf("claim: %d %s", result.Code, result.Body.String())
	}
	var created struct {
		Data logic.CreateResult `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.EditCode == "" {
		t.Fatal("missing owner code")
	}
	for _, path := range []string{"/api/v1/admin/growth", "/api/v1/admin/growth?source=class_invite"} {
		if response := request("GET", path, "", ""); response.Code != 401 {
			t.Fatalf("public metrics: %d", response.Code)
		}
	}
	metrics := request("GET", "/api/v1/admin/growth?source=class_invite&campaign=reunion", "", token)
	validMetrics := metrics.Code == 200 && strings.Contains(metrics.Body.String(), `"registration_success":1`) &&
		strings.Contains(metrics.Body.String(), `"share_registration":1`)
	if !validMetrics {
		t.Fatalf("metrics: %d %s", metrics.Code, metrics.Body.String())
	}
	forged := request("POST", "/api/v1/growth/events", `{"event":"registration_success","visitor_id":"fake"}`, "")
	if forged.Code != http.StatusBadRequest {
		t.Fatalf("forged success: %d", forged.Code)
	}
	page := request("GET", "/class?graduation_year=2018&class_name=test", "", "")
	if page.Code != 200 || !strings.Contains(page.Body.String(), "<html") {
		t.Fatalf("class route: %d", page.Code)
	}
}
