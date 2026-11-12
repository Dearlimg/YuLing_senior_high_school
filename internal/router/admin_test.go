package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"YuLing_senior_high_school/internal/logic"
)

func TestAdminRolesSoftDeleteAndImmediateRevocation(t *testing.T) {
	const bootstrap = "test-bootstrap-token-for-local-tests-only"
	router := testEngineWithAdmin(bootstrap)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	check := func(rec *httptest.ResponseRecorder, status int) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("want %d got %d: %s", status, rec.Code, rec.Body.String())
		}
	}
	register := func(account string) logic.AuthResult {
		rec := call("POST", "/auth/register", "", fmt.Sprintf(`{"account":%q,"password":"password123","name":%q,"graduation_year":2018,"is_admin":true}`, account, account))
		check(rec, 200)
		var out struct {
			Data logic.AuthResult `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Data
	}
	a := register("alice_admin")
	b := register("bob_member")
	check(call("GET", "/admin/users", a.Token, ""), 403)
	check(call("PUT", "/admin/users/1/role", a.Token, `{"is_admin":true,"reason":"self elevation"}`), 403)
	check(call("PUT", "/admin/users/1/role", bootstrap, `{"is_admin":true,"reason":"initial administrator"}`), 200)
	me := call("GET", "/me", a.Token, "")
	check(me, 200)
	if !strings.Contains(me.Body.String(), `"is_admin":true`) {
		t.Fatal("existing session did not gain role")
	}
	check(call("PUT", "/admin/users/2/role", a.Token, `{"is_admin":true,"reason":"second administrator"}`), 200)
	check(call("GET", "/admin/users", b.Token, ""), 200)
	check(call("PUT", "/admin/users/2/role", b.Token, `{"is_admin":false,"reason":"self removal"}`), 400)
	check(call("PUT", "/admin/users/2/role", a.Token, `{"reason":"missing state"}`), 400)
	check(call("PUT", "/admin/users/999/role", a.Token, `{"is_admin":true,"reason":"unknown user"}`), 404)

	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	create := func(path string, photo bool) uint64 {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if photo {
			_ = writer.WriteField("caption", "测试照片")
			_ = writer.WriteField("category", "校园风景")
		} else {
			_ = writer.WriteField("content", "测试帖子")
		}
		part, err := writer.CreateFormFile("image", "campus.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(imageBytes.Bytes())
		_ = writer.Close()
		req := httptest.NewRequest("POST", "/api/v1"+path, &body)
		req.Header.Set("Authorization", "Bearer "+b.Token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		check(rec, 200)
		var out struct {
			Data struct {
				ID uint64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Data.ID
	}
	post := create("/posts", false)
	photo := create("/photos", true)
	for _, target := range []struct {
		kind, collection string
		id               uint64
	}{{"post", "posts", post}, {"photo", "photos", photo}} {
		path := fmt.Sprintf("/%s/%d", target.collection, target.id)
		check(call("POST", path+"/replies", b.Token, `{"content":"测试评论"}`), 200)
		check(call("PUT", path+"/likes", b.Token, `{"liked":true}`), 200)
		check(call("GET", path, "", ""), 200)
		adminPath := fmt.Sprintf("/admin/%s/%d", target.kind, target.id)
		check(call("PUT", adminPath, a.Token, `{"action":"hidden","reason":"review"}`), 200)
		check(call("GET", path+"/image", "", ""), 404)
		check(call("PUT", adminPath, a.Token, `{"action":"active","reason":"approved"}`), 200)
		check(call("GET", path+"/image", "", ""), 200)
		check(call("DELETE", adminPath, a.Token, `{"reason":"soft delete test"}`), 200)
		for _, suffix := range []string{"", "/image", "/replies", "/likes"} {
			check(call("GET", path+suffix, b.Token, ""), 404)
		}
		check(call("POST", path+"/replies", b.Token, `{"content":"after deletion"}`), 404)
		check(call("PUT", path+"/likes", b.Token, `{"liked":true}`), 404)
		stored := call("GET", adminPath, a.Token, "")
		check(stored, 200)
		if !strings.Contains(stored.Body.String(), `"status":"deleted"`) {
			t.Fatal("soft deleted row was not retained")
		}
		check(call("PUT", adminPath, a.Token, `{"action":"active","reason":"cannot resurrect deletion"}`), 404)
		listing := call("GET", "/admin/content/"+target.kind+"?status=deleted", a.Token, "")
		check(listing, 200)
		if !strings.Contains(listing.Body.String(), `"total":1`) {
			t.Fatal("deleted item absent from management list")
		}
	}
	users := call("GET", "/admin/users?keyword=bob", a.Token, "")
	check(users, 200)
	for _, forbidden := range []string{"PasswordHash", "password_hash", "password123"} {
		if strings.Contains(users.Body.String(), forbidden) {
			t.Fatal("credentials in admin user response")
		}
	}
	check(call("PUT", "/admin/users/2/role", a.Token, `{"is_admin":false,"reason":"revoke access"}`), 200)
	check(call("GET", "/admin/users", b.Token, ""), 403)
	check(call("DELETE", fmt.Sprintf("/admin/post/%d", post), b.Token, `{"reason":"revoked admin"}`), 403)
	logs := call("GET", "/admin/audit", a.Token, "")
	check(logs, 200)
	if !strings.Contains(logs.Body.String(), `"ActorUserID":1`) || !strings.Contains(logs.Body.String(), `revoke_admin`) {
		t.Fatal("actor and role changes absent from audit")
	}
}
