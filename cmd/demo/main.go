package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/jobs"
	"strings"
	"time"
)

var client = &http.Client{Timeout: 15 * time.Second}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func call(method, path string, in any, token string) (int, map[string]any) {
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		must(e)
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
	out := map[string]any{}
	json.Unmarshal(b, &out)
	return resp.StatusCode, out
}
func login(name string) string {
	resp, e := client.Post("http://127.0.0.1:8000/auth/login", "application/x-www-form-urlencoded", strings.NewReader(url.Values{"username": {name + "@example.com"}, "password": {"SiriusDemo2026!"}}.Encode()))
	must(e)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Fatalf("login %d: %s", resp.StatusCode, b)
	}
	var pair map[string]string
	must(json.Unmarshal(b, &pair))
	return pair["access_token"]
}
func main() {
	cfg := config.Load()
	ctx := context.Background()
	p, e := database.Open(ctx, cfg.DatabaseURL)
	must(e)
	defer p.Close()
	engine := jobs.New(p, cfg)
	defer engine.Close()
	token := login("student")
	result := map[string]any{"language": "Go", "time": time.Now().UTC()}
	code, _ := call("POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, "")
	result["no_token_http"] = code
	begin := time.Now()
	code, order := call("POST", "/orders/create", map[string]any{"item_id": 1, "quantity": 1}, token)
	if code != 200 {
		log.Fatal(code, order)
	}
	result["api_ms"] = float64(time.Since(begin).Microseconds()) / 1000
	oid := int64(order["order_id"].(float64))
	result["order_id"] = oid
	deadline := time.Now().Add(30 * time.Second)
	var done int
	for time.Now().Before(deadline) {
		must(p.QueryRow(ctx, "SELECT count(*)FROM task_logs WHERE order_id=$1 AND state='done'", oid).Scan(&done))
		if done == 3 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if done != 3 {
		log.Fatal("worker did not finish three jobs")
	}
	result["effects_done"] = done
	var job int64
	must(p.QueryRow(ctx, "SELECT id FROM task_logs WHERE order_id=$1 ORDER BY id LIMIT 1", oid).Scan(&job))
	duplicate, e := engine.Enqueue(ctx, job, "")
	must(e)
	inspector := asynq.NewInspector(engine.Opt)
	defer inspector.Close()
	wait := func(id string, want asynq.TaskState) *asynq.TaskInfo {
		end := time.Now().Add(30 * time.Second)
		for time.Now().Before(end) {
			info, e := inspector.GetTaskInfo("sirius", id)
			if e == nil && info.State == want {
				return info
			}
			time.Sleep(100 * time.Millisecond)
		}
		log.Fatal("task timeout", id, want)
		return nil
	}
	result["duplicate_result"] = json.RawMessage(wait(duplicate.ID, asynq.TaskStateCompleted).Result)
	makeJob := func() int64 {
		var order, id int64
		must(p.QueryRow(ctx, "INSERT INTO orders(user_id,item_id,quantity,total_cents)VALUES(1,1,1,15000)RETURNING id").Scan(&order))
		must(p.QueryRow(ctx, "INSERT INTO task_logs(order_id,kind,state)VALUES($1,'sms','queued')RETURNING id", order).Scan(&id))
		return id
	}
	once, permanent := makeJob(), makeJob()
	a, e := engine.Enqueue(ctx, once, "once")
	must(e)
	b, e := engine.Enqueue(ctx, permanent, "permanent")
	must(e)
	wait(a.ID, asynq.TaskStateCompleted)
	archived := wait(b.ID, asynq.TaskStateArchived)
	result["archived_task_id"] = archived.ID
	result["archived_retries"] = archived.Retried
	attempts := map[string]any{}
	for _, id := range []int64{job, once, permanent} {
		var state string
		var count int
		must(p.QueryRow(ctx, "SELECT state,attempts FROM task_logs WHERE id=$1", id).Scan(&state, &count))
		attempts[fmt.Sprint(id)] = map[string]any{"state": state, "attempts": count}
	}
	result["attempts"] = attempts
	raw, e := engine.Redis.LRange(ctx, "sirius:dlq", 0, -1).Result()
	must(e)
	dlq := []json.RawMessage{}
	found := false
	for _, v := range raw {
		var item map[string]any
		json.Unmarshal([]byte(v), &item)
		if int64(item["job_id"].(float64)) == permanent {
			found = true
			dlq = append(dlq, json.RawMessage(v))
		}
	}
	if !found {
		log.Fatal("missing DLQ record")
	}
	result["dlq"] = dlq
	bjson, e := json.MarshalIndent(result, "", "  ")
	must(e)
	must(os.MkdirAll("evidence", 0755))
	must(os.WriteFile(filepath.Join("evidence", "worker-demo.json"), bjson, 0644))
	fmt.Println(string(bjson))
}
