package app_test

import (
	"context"
	"encoding/json"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/jobs"
	"sync"
	"testing"
	"time"
)

func fixtureJob(t *testing.T, p *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var oid, id int64
	if e := p.QueryRow(ctx, "INSERT INTO orders(user_id,item_id,quantity,total_cents)VALUES(1,1,1,15000)RETURNING id").Scan(&oid); e != nil {
		t.Fatal(e)
	}
	if e := p.QueryRow(ctx, "INSERT INTO task_logs(order_id,kind,state)VALUES($1,'sms','queued')RETURNING id", oid).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}
func engineFor(t *testing.T, p *pgxpool.Pool) *jobs.Engine {
	t.Helper()
	cfg := config.Load()
	cfg.TasksMode = "async"
	engine := jobs.New(p, cfg)
	engine.Redis.FlushDB(context.Background())
	t.Cleanup(engine.Close)
	return engine
}
func TestJobsIdempotency(t *testing.T) {

	_, p := setup(t)
	e := engineFor(t, p)
	e.Delays["sms"] = 10 * time.Millisecond
	id := fixtureJob(t, p)
	var wg sync.WaitGroup
	results := make(chan string, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			state, err := e.Process(context.Background(), jobs.Payload{JobID: id}, 0)
			results <- state
			errs <- err
		})
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]int{}
	for state := range results {
		seen[state]++
	}
	var attempts int
	p.QueryRow(context.Background(), "SELECT attempts FROM task_logs WHERE id=$1", id).Scan(&attempts)
	if seen["done"] != 1 || seen["already_done"] != 1 || attempts != 1 {
		t.Fatal(seen, attempts)
	}
}
func TestJobsRetryArchiveAndDLQ(t *testing.T) {

	_, p := setup(t)
	engine := engineFor(t, p)
	engine.Delays["sms"] = 10 * time.Millisecond
	server, mux := engine.Worker()
	if err := server.Start(mux); err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown()
	ctx := context.Background()
	transient, permanent := fixtureJob(t, p), fixtureJob(t, p)
	first, err := engine.Enqueue(ctx, transient, "once")
	if err != nil {
		t.Fatal(err)
	}
	last, err := engine.Enqueue(ctx, permanent, "permanent")
	if err != nil {
		t.Fatal(err)
	}
	inspector := asynq.NewInspector(engine.Opt)
	defer inspector.Close()
	deadline := time.Now().Add(35 * time.Second)
	var success, archived bool
	for time.Now().Before(deadline) {
		a, e1 := inspector.GetTaskInfo("sirius", first.ID)
		b, e2 := inspector.GetTaskInfo("sirius", last.ID)
		success = e1 == nil && a.State == asynq.TaskStateCompleted
		archived = e2 == nil && b.State == asynq.TaskStateArchived
		if success && archived {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !success || !archived {
		t.Fatal("retry or archive failed", success, archived)
	}
	var attempts int
	var state string
	p.QueryRow(ctx, "SELECT attempts,state FROM task_logs WHERE id=$1", permanent).Scan(&attempts, &state)
	if attempts != 4 || state != "failed" {
		t.Fatal(attempts, state)
	}
	p.QueryRow(ctx, "SELECT attempts FROM task_logs WHERE id=$1", transient).Scan(&attempts)
	if attempts != 2 {
		t.Fatal(attempts)
	}
	raw, err := engine.Redis.LRange(ctx, "sirius:dlq", 0, -1).Result()
	if err != nil || len(raw) != 1 {
		t.Fatal(raw, err)
	}
	var audit map[string]any
	json.Unmarshal([]byte(raw[0]), &audit)
	if int64(audit["job_id"].(float64)) != permanent {
		t.Fatal(audit)
	}
}
func TestScheduledStatisticsHandler(t *testing.T) {

	_, p := setup(t)
	e := engineFor(t, p)
	if err := e.Statistics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.Redis.Exists(context.Background(), "sirius:statistics:"+time.Now().UTC().Format("2006-01-02")).Val() != 1 {
		t.Fatal("statistics not stored")
	}
}
