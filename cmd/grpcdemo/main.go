package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/rpcdelivery"
	"strings"
	"time"
)

func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

var client = &http.Client{Timeout: 15 * time.Second}

func call(method, path string, in any, token string) (int, map[string]any) {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	r, e := http.NewRequest(method, "http://127.0.0.1:8000"+path, body)
	must(e)
	if in != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, e := client.Do(r)
	must(e)
	defer resp.Body.Close()
	b, e := io.ReadAll(resp.Body)
	must(e)
	var out map[string]any
	json.Unmarshal(b, &out)
	return resp.StatusCode, out
}
func login(name string) (string, domain.User) {
	resp, e := client.Post("http://127.0.0.1:8000/auth/login", "application/x-www-form-urlencoded", strings.NewReader(url.Values{"username": {name + "@example.com"}, "password": {"SiriusDemo2026!"}}.Encode()))
	must(e)
	defer resp.Body.Close()
	var pair map[string]string
	must(json.NewDecoder(resp.Body).Decode(&pair))
	if resp.StatusCode != 200 {
		log.Fatal(resp.StatusCode, pair)
	}
	token := pair["access_token"]
	code, u := call("GET", "/users/me", nil, token)
	if code != 200 {
		log.Fatal(code, u)
	}
	return token, domain.User{ID: int64(u["id"].(float64)), Email: u["email"].(string), Role: u["role"].(string)}
}
func main() {
	cfg := config.Load()
	student, actor := login("student")
	other, _ := login("student2")
	cook, _ := login("cook")
	courier, worker := login("courier")
	result := map[string]any{"language": "Go", "time": time.Now().UTC()}
	code, _ := call("POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, "")
	result["no_token_http"] = code
	code, _ = call("GET", "/orders/kitchen/orders", nil, student)
	result["wrong_role_http"] = code
	code, o := call("POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, student)
	if code != 200 {
		log.Fatal(code, o)
	}
	oid := int64(o["order_id"].(float64))
	result["order_id"] = oid
	code, _ = call("POST", fmt.Sprintf("/orders/%d/cancel", oid), nil, other)
	result["foreign_order_http"] = code
	code, out := call("POST", fmt.Sprintf("/orders/%d/paid", oid), nil, cook)
	if code != 200 {
		log.Fatal(code, out)
	}
	code, d := call("POST", "/delivery/assign", map[string]any{"order_id": oid, "courier_id": worker.ID}, cook)
	if code != 200 {
		log.Fatal(code, d)
	}
	result["assigned_delivery"] = d
	did := int64(d["delivery_id"].(float64))
	code, _ = call("GET", fmt.Sprintf("/delivery/orders/%d", oid), nil, other)
	result["foreign_delivery_http"] = code
	gateway, e := rpcdelivery.NewClient(cfg)
	must(e)
	defer gateway.Close()
	statuses, e := gateway.Track(context.Background(), oid, actor)
	must(e)
	result["server_stream"] = statuses
	replies, e := gateway.Chat(context.Background(), oid, actor, []string{"Где мой заказ?", "Спасибо!"})
	must(e)
	result["bidirectional_chat"] = replies
	code, out = call("PATCH", fmt.Sprintf("/delivery/%d/complete", did), nil, courier)
	if code != 200 {
		log.Fatal(code, out)
	}
	code, out = call("GET", fmt.Sprintf("/delivery/orders/%d", oid), nil, student)
	if code != 200 {
		log.Fatal(code, out)
	}
	result["completed_delivery"] = out
	for _, k := range []string{"foreign_order_http", "foreign_delivery_http", "wrong_role_http"} {
		if result[k] != 403 {
			log.Fatal(k, result[k])
		}
	}
	b, e := json.MarshalIndent(result, "", "  ")
	must(e)
	must(os.MkdirAll("evidence", 0755))
	must(os.WriteFile("evidence/grpc-demo.json", b, 0644))
	fmt.Println(string(b))
}
