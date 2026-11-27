package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/database"
	"sirius.local/food/internal/jobs"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: loadcleanup fixture-user-id")
	}
	user, e := strconv.ParseInt(os.Args[1], 10, 64)
	if e != nil {
		log.Fatal(e)
	}
	cfg := config.Load()
	ctx := context.Background()
	p, e := database.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer p.Close()
	var email string
	if e = p.QueryRow(ctx, "SELECT email FROM users WHERE id=$1", user).Scan(&email); e != nil {
		log.Fatal(e)
	}
	if !strings.HasPrefix(email, "load-") || !strings.HasSuffix(email, "@example.com") {
		log.Fatal("refusing non-load fixture")
	}
	rows, e := p.Query(ctx, "SELECT t.id FROM task_logs t JOIN orders o ON o.id=t.order_id WHERE o.user_id=$1 AND t.state<>'done'", user)
	if e != nil {
		log.Fatal(e)
	}
	ids := map[int64]bool{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			log.Fatal(e)
		}
		ids[id] = true
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		log.Fatal(e)
	}
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: config.Int("ASYNQ_DB", 1)})
	defer inspector.Close()
	list := []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){inspector.ListPendingTasks, inspector.ListScheduledTasks, inspector.ListRetryTasks, inspector.ListArchivedTasks}
	taskIDs := []string{}
	for _, fn := range list {
		for page := 1; ; page++ {
			tasks, e := fn("sirius", asynq.Page(page), asynq.PageSize(1000))
			if e != nil {
				log.Fatal(e)
			}
			for _, task := range tasks {
				var payload jobs.Payload
				if task.Type == jobs.EffectTask && json.Unmarshal(task.Payload, &payload) == nil && ids[payload.JobID] {
					taskIDs = append(taskIDs, task.ID)
				}
			}
			if len(tasks) < 1000 {
				break
			}
		}
	}
	for _, id := range taskIDs {
		if e = inspector.DeleteTask("sirius", id); e != nil {
			log.Fatal(e)
		}
	}
	tag, e := p.Exec(ctx, "UPDATE task_logs t SET state='cancelled',result='Load fixture stopped after measurement',updated_at=now() FROM orders o WHERE t.order_id=o.id AND o.user_id=$1 AND t.state<>'done'", user)
	if e != nil {
		log.Fatal(e)
	}
	fmt.Printf("fixture_user=%d cancelled_background_jobs=%d removed_queue_tasks=%d\n", user, tag.RowsAffected(), len(taskIDs))
}
