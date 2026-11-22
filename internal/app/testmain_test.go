package app_test

import (
	"context"
	"fmt"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) { os.Exit(runTests(m)) }
func runTests(m *testing.M) int {
	if os.Getenv("SIRIUS_TEST_DB") != "" {
		return m.Run()
	}
	ctx := context.Background()
	pg, e := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "postgres:15-alpine", Env: map[string]string{"POSTGRES_USER": "sirius", "POSTGRES_PASSWORD": "sirius", "POSTGRES_DB": "sirius_test"}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90 * time.Second)}, Started: true})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer pg.Terminate(ctx)
	host, e := pg.Host(ctx)
	if e != nil {
		return 1
	}
	port, e := pg.MappedPort(ctx, "5432/tcp")
	if e != nil {
		return 1
	}
	os.Setenv("SIRIUS_TEST_DB", fmt.Sprintf("postgres://sirius:sirius@%s:%s/sirius_test?sslmode=disable", host, port.Port()))
	redis, e := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForListeningPort("6379/tcp")}, Started: true})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer redis.Terminate(ctx)
	host, e = redis.Host(ctx)
	if e != nil {
		return 1
	}
	port, e = redis.MappedPort(ctx, "6379/tcp")
	if e != nil {
		return 1
	}
	os.Setenv("REDIS_ADDR", host+":"+port.Port())
	return m.Run()
}
