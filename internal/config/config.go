package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const Stage = 10

type Config struct {
	DatabaseURL, RedisAddr, HTTPAddr, JWTSecret, TasksMode, GRPCAddr, ServiceToken string
	Debug                                                                          bool
	Cache                                                                          bool
}

func Load() Config {
	if f, e := os.Open(".env"); e == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			k, v, ok := strings.Cut(s.Text(), "=")
			if ok && os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
	return Config{DatabaseURL: env("DATABASE_URL", "postgres://sirius:sirius@127.0.0.1:55432/sirius_go?sslmode=disable"), RedisAddr: env("REDIS_ADDR", "127.0.0.1:56379"), HTTPAddr: env("HTTP_ADDR", "127.0.0.1:8000"), JWTSecret: os.Getenv("JWT_SECRET"), TasksMode: env("TASKS_MODE", "async"), GRPCAddr: env("GRPC_ADDR", "127.0.0.1:50051"), ServiceToken: os.Getenv("SERVICE_TOKEN"), Debug: env("DEBUG", "true") == "true", Cache: env("CACHE_ENABLED", "true") == "true"}

}
func env(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}
func Int(k string, v int) int {
	if x, e := strconv.Atoi(os.Getenv(k)); e == nil {
		return x
	}
	return v
}
