package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sirius.local/food/internal/app"
	"sirius.local/food/internal/config"
	"strings"
	"testing"
)

func login(t *testing.T, s *httptest.Server, name string) (map[string]string, map[string]any) {
	t.Helper()
	form := url.Values{"username": {name + "@example.com"}, "password": {"SiriusDemo2026!"}}
	resp, e := s.Client().Post(s.URL+"/auth/login", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var pair map[string]any
	json.Unmarshal(b, &pair)
	if resp.StatusCode != 200 {
		t.Fatalf("login: %s", b)
	}
	return map[string]string{"Authorization": "Bearer " + pair["access_token"].(string)}, pair
}
func TestAuthPermissions(t *testing.T) {

	s, _ := setup(t)
	h, _ := login(t, s, "student")
	other, _ := login(t, s, "student2")
	code, _ := request(t, s, "POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, nil)
	if code != 401 {
		t.Fatal(code)
	}
	code, _ = request(t, s, "GET", "/orders/kitchen/orders", nil, h)
	if code != 403 {
		t.Fatal(code)
	}
	code, o := request(t, s, "POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, h)
	if code != 200 {
		t.Fatal(code, o)
	}
	oid := int64(o["order_id"].(float64))
	code, _ = request(t, s, "POST", "/orders/"+strconvID(oid)+"/cancel", nil, other)
	if code != 403 {
		t.Fatal(code)
	}
	code, _ = request(t, s, "GET", "/orders/"+strconvID(oid), nil, h)
	if code != 200 {
		t.Fatal(code)
	}
	code, _ = request(t, s, "POST", "/users/1/topup?amount=100", nil, other)
	if code != 403 {
		t.Fatal(code)
	}
}
func TestRateLimit(t *testing.T) {

	s, _ := setup(t)
	for i := 0; i < 6; i++ {
		resp, e := s.Client().Post(s.URL+"/auth/login", "application/x-www-form-urlencoded", strings.NewReader("username=nobody%40example.com&password=wrong"))
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		want := 401
		if i == 5 {
			want = 429
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d: %d", i, resp.StatusCode)
		}
	}
}
func TestRefreshReplay(t *testing.T) {

	s, _ := setup(t)
	_, first := login(t, s, "student")
	code, next := request(t, s, "POST", "/auth/refresh", map[string]any{"refresh_token": first["refresh_token"]}, nil)
	if code != 200 {
		t.Fatal(code, next)
	}
	if first["refresh_token"] == next["refresh_token"] {
		t.Fatal("not rotated")
	}
	code, _ = request(t, s, "POST", "/auth/refresh", map[string]any{"refresh_token": first["refresh_token"]}, nil)
	if code != 401 {
		t.Fatal(code)
	}
	code, _ = request(t, s, "GET", "/users/me", nil, map[string]string{"Authorization": "Bearer " + next["access_token"].(string)})
	if code != 401 {
		t.Fatal(code)
	}
}
func TestPasswordHashAndRole(t *testing.T) {

	s, p := setup(t)
	code, u := request(t, s, "POST", "/users/register", map[string]any{"email": "new@example.com", "password": "StrongPass123!"}, nil)
	if code != 200 {
		t.Fatal(code, u)
	}
	var hash, role string
	p.QueryRow(context.Background(), "SELECT password_hash,role FROM users WHERE email='new@example.com'").Scan(&hash, &role)
	if !strings.HasPrefix(hash, "$2a$") || role != "student" {
		t.Fatal(hash, role)
	}
	code, _ = request(t, s, "POST", "/users/register", map[string]any{"email": "admin2@example.com", "password": "StrongPass123!", "role": "admin"}, nil)
	if code != 400 {
		t.Fatal(code)
	}
}
func TestRefundOnlyOnce(t *testing.T) {

	s, _ := setup(t)
	h, _ := login(t, s, "student")
	_, u := request(t, s, "GET", "/users/me", nil, h)
	before := u["balance"]
	code, o := request(t, s, "POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, h)
	if code != 200 {
		t.Fatal(code, o)
	}
	path := "/orders/" + strconvID(int64(o["order_id"].(float64))) + "/cancel"
	code, _ = request(t, s, "POST", path, nil, h)
	if code != 200 {
		t.Fatal(code)
	}
	code, _ = request(t, s, "POST", path, nil, h)
	if code != 400 {
		t.Fatal(code)
	}
	_, u = request(t, s, "GET", "/users/me", nil, h)
	if before != u["balance"] {
		t.Fatal(before, u)
	}
}
func TestProductionDocsDisabled(t *testing.T) {

	s, p := setup(t)
	_ = s
	cfg := config.Load()
	cfg.JWTSecret = "test-secret-longer-than-thirty-two-characters"
	cfg.Debug = false
	h, closeFn, e := app.New(context.Background(), p, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer closeFn()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/docs", nil))
	if rr.Code != 404 {
		t.Fatal(rr.Code)
	}
}
func strconvID(id int64) string { return fmt.Sprint(id) }

var _ = http.MethodGet

func TestDeliveryOwnership(t *testing.T) {

	s, _ := setup(t)
	student, _ := login(t, s, "student")
	cook, _ := login(t, s, "cook")
	courier, _ := login(t, s, "courier")
	other, _ := login(t, s, "student2")
	wrongCourier, _ := login(t, s, "courier2")
	code, o := request(t, s, "POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, student)
	if code != 200 {
		t.Fatal(code, o)
	}
	oid := strconvID(int64(o["order_id"].(float64)))
	code, _ = request(t, s, "POST", "/orders/"+oid+"/paid", nil, cook)
	if code != 200 {
		t.Fatal(code)
	}
	code, d := request(t, s, "POST", "/delivery/assign", map[string]any{"order_id": o["order_id"], "courier_id": 4}, cook)
	if code != 200 {
		t.Fatal(code, d)
	}
	code, _ = request(t, s, "GET", "/delivery/orders/"+oid, nil, other)
	if code != 403 {
		t.Fatal(code)
	}
	did := strconvID(int64(d["delivery_id"].(float64)))
	code, _ = request(t, s, "PATCH", "/delivery/"+did+"/complete", nil, wrongCourier)
	if code != 403 {
		t.Fatal(code)
	}
	code, _ = request(t, s, "PATCH", "/delivery/"+did+"/complete", nil, courier)
	if code != 200 {
		t.Fatal(code)
	}
}
