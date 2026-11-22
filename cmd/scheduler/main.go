package main

import (
	"github.com/hibiken/asynq"
	"log"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/jobs"
	"time"
)

func main() {
	cfg := config.Load()
	opt := asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: config.Int("ASYNQ_DB", 1)}
	scheduler := asynq.NewScheduler(opt, &asynq.SchedulerOpts{Location: time.UTC})
	if _, e := scheduler.Register("@every 1m", asynq.NewTask(jobs.StatisticsTask, nil), asynq.Queue("sirius")); e != nil {
		log.Fatal(e)
	}
	if _, e := scheduler.Register("@every 10s", asynq.NewTask(jobs.OutboxTask, nil), asynq.Queue("sirius")); e != nil {
		log.Fatal(e)
	}
	log.Print("Go scheduler started: statistics every minute; outbox every 10 seconds")
	if e := scheduler.Run(); e != nil {
		log.Fatal(e)
	}
}
