package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log"
	"sirius.local/food/internal/config"
	"time"
)

const EffectTask = "sirius:effect"
const StatisticsTask = "sirius:statistics"
const OutboxTask = "sirius:outbox"

type Payload struct {
	JobID   int64  `json:"job_id"`
	Failure string `json:"failure_mode,omitempty"`
}
type Engine struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Client *asynq.Client
	Opt    asynq.RedisClientOpt
	Mode   string
	Delays map[string]time.Duration
}

func New(p *pgxpool.Pool, cfg config.Config) *Engine {
	opt := asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: config.Int("ASYNQ_DB", 1), DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second}
	return &Engine{p, redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, DB: opt.DB}), asynq.NewClient(opt), opt, cfg.TasksMode, map[string]time.Duration{"pdf": 2 * time.Second, "email": time.Second, "sms": time.Second}}
}
func (e *Engine) Close() { e.Client.Close(); e.Redis.Close() }
func (e *Engine) Enqueue(ctx context.Context, id int64, failure string) (*asynq.TaskInfo, error) {
	b, _ := json.Marshal(Payload{id, failure})
	return e.Client.EnqueueContext(ctx, asynq.NewTask(EffectTask, b), asynq.Queue("sirius"), asynq.MaxRetry(3), asynq.Retention(time.Hour), asynq.Timeout(30*time.Second))
}
func (e *Engine) AfterOrder(ctx context.Context, id int64) error {
	if e.Mode == "disabled" {
		return nil
	}
	if e.Mode == "sync" {
		rows, err := e.Pool.Query(ctx, "SELECT id FROM task_logs WHERE order_id=$1 ORDER BY id", id)
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, job := range ids {
			if _, err = e.Process(ctx, Payload{JobID: job}, 0); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := e.Publish(ctx, id)
	if err != nil {
		log.Printf("outbox retained order=%d: %v", id, err)
	}
	return nil
}
func (e *Engine) Publish(ctx context.Context, order int64) (int, error) {
	rows, err := e.Pool.Query(ctx, "SELECT id FROM task_logs WHERE ($1::bigint=0 OR order_id=$1)AND(state='pending' OR(state='queued' AND updated_at<now()-interval '2 minutes'))ORDER BY id", order)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	n := 0
	for _, id := range ids {
		tag, err := e.Pool.Exec(ctx, "UPDATE task_logs SET state='queued',updated_at=now() WHERE id=$1 AND(state='pending'OR(state='queued'AND updated_at<now()-interval '2 minutes'))", id)
		if err != nil {
			return n, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		if _, err = e.Enqueue(ctx, id, ""); err != nil {
			e.Pool.Exec(ctx, "UPDATE task_logs SET state='pending'WHERE id=$1 AND state='queued'", id)
			return n, err
		}
		n++
	}
	return n, nil
}
func (e *Engine) Process(ctx context.Context, p Payload, retry int) (string, error) {
	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var kind, state string
	var order int64
	err = tx.QueryRow(ctx, "SELECT kind,state,order_id FROM task_logs WHERE id=$1 FOR UPDATE", p.JobID).Scan(&kind, &state, &order)
	if err != nil {
		return "", fmt.Errorf("unknown job %d: %w", p.JobID, asynq.SkipRetry)
	}
	if state == "done" {
		return "already_done", nil
	}
	if _, err = tx.Exec(ctx, "UPDATE task_logs SET attempts=attempts+1,updated_at=now() WHERE id=$1", p.JobID); err != nil {
		return "", err
	}
	if p.Failure == "permanent" || p.Failure == "once" && retry == 0 {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", fmt.Errorf("учебный отказ провайдера")
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(e.Delays[kind]):
	}
	result := fmt.Sprintf("SIMULATED %s for order %d", kind, order)
	if _, err = tx.Exec(ctx, "UPDATE task_logs SET state='done',result=$2,updated_at=now()WHERE id=$1", p.JobID, result); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	log.Printf("EFFECT job=%d kind=%s order=%d", p.JobID, kind, order)
	return "done", nil
}
func (e *Engine) Handle(ctx context.Context, t *asynq.Task) error {
	var p Payload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("payload: %w", asynq.SkipRetry)
	}
	retry, _ := asynq.GetRetryCount(ctx)
	state, err := e.Process(ctx, p, retry)
	if err == nil {
		b, _ := json.Marshal(map[string]any{"state": state, "job_id": p.JobID})
		t.ResultWriter().Write(b)
	}
	return err
}
func (e *Engine) OnError(ctx context.Context, t *asynq.Task, err error) {
	if t.Type() != EffectTask {
		return
	}
	var p Payload
	json.Unmarshal(t.Payload(), &p)
	retry, _ := asynq.GetRetryCount(ctx)
	max, _ := asynq.GetMaxRetry(ctx)
	state := "retry"
	if retry >= max || errors.Is(err, asynq.SkipRetry) {
		state = "failed"
	}
	e.Pool.Exec(ctx, "UPDATE task_logs SET state=$2,result=$3,updated_at=now()WHERE id=$1 AND state<>'done'", p.JobID, state, err.Error())
	b, _ := json.Marshal(map[string]any{"job_id": p.JobID, "retry": retry, "state": state, "error": err.Error(), "time": time.Now().UTC()})
	e.Redis.RPush(ctx, "sirius:retry_events", b)
	if state == "failed" {
		e.Redis.RPush(ctx, "sirius:dlq", b)
	}
}
func (e *Engine) Statistics(ctx context.Context) error {
	var count int64
	if err := e.Pool.QueryRow(ctx, "SELECT count(*)FROM orders WHERE created_at>=date_trunc('day',now()AT TIME ZONE 'UTC')AT TIME ZONE 'UTC'").Scan(&count); err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"orders_today": count, "updated_at": time.Now().UTC()})
	return e.Redis.Set(ctx, "sirius:statistics:"+time.Now().UTC().Format("2006-01-02"), b, 48*time.Hour).Err()
}
func (e *Engine) Worker() (*asynq.Server, *asynq.ServeMux) {
	srv := asynq.NewServer(e.Opt, asynq.Config{Concurrency: 4, Queues: map[string]int{"sirius": 1}, TaskCheckInterval: 100 * time.Millisecond, DelayedTaskCheckInterval: 200 * time.Millisecond, ShutdownTimeout: 10 * time.Second, RetryDelayFunc: func(n int, err error, t *asynq.Task) time.Duration {
		if n > 6 {
			n = 6
		}
		return time.Second << n
	}, ErrorHandler: asynq.ErrorHandlerFunc(e.OnError)})
	mux := asynq.NewServeMux()
	mux.HandleFunc(EffectTask, e.Handle)
	mux.HandleFunc(StatisticsTask, func(ctx context.Context, t *asynq.Task) error { return e.Statistics(ctx) })
	mux.HandleFunc(OutboxTask, func(ctx context.Context, t *asynq.Task) error { _, err := e.Publish(ctx, 0); return err })
	return srv, mux
}
