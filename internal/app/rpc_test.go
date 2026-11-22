package app_test

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"net/http/httptest"
	"sirius.local/food/internal/app"
	"sirius.local/food/internal/config"
	pb "sirius.local/food/internal/deliverypb"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/rpcdelivery"
	"testing"
	"time"
)

func TestRPCStreamsAndInternalKey(t *testing.T) {

	_, p := setup(t)
	ctx := context.Background()
	cfg := config.Load()
	cfg.ServiceToken = "test-rpc-key-longer-than-thirty-two-characters"
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	cfg.GRPCAddr = listener.Addr().String()
	srv := rpcdelivery.NewServer(p, cfg)
	go srv.Serve(listener)
	defer srv.Stop()
	gateway, e := rpcdelivery.NewClient(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer gateway.Close()
	var oid int64
	e = p.QueryRow(ctx, "INSERT INTO orders(user_id,item_id,quantity,total_cents,status)VALUES(1,1,1,15000,'paid')RETURNING id").Scan(&oid)
	if e != nil {
		t.Fatal(e)
	}
	cook := domain.User{ID: 3, Role: "cook"}
	student := domain.User{ID: 1, Role: "student"}
	if _, e = gateway.Assign(ctx, oid, 4, &cook); e != nil {
		t.Fatal(e)
	}
	stream, e := gateway.Track(ctx, oid, student)
	if e != nil {
		t.Fatal(e)
	}
	if len(stream) != 3 || stream[0].Status != "in_progress" || stream[2].Status != "delivered" || !stream[0].Simulated {
		t.Fatal(stream)
	}
	replies, e := gateway.Chat(ctx, oid, student, []string{"Где заказ?", "Спасибо"})
	if e != nil || len(replies) != 2 {
		t.Fatal(replies, e)
	}
	_, e = gateway.Stub.GetDelivery(ctx, &pb.GetDeliveryRequest{OrderId: oid, ActorId: 1})
	if status.Code(e) != codes.Unauthenticated {
		t.Fatal(e)
	}
}

type slowServer struct {
	pb.UnimplementedDeliveryServiceServer
}

func (slowServer) GetDelivery(ctx context.Context, q *pb.GetDeliveryRequest) (*pb.GetDeliveryResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(200 * time.Millisecond):
		return &pb.GetDeliveryResponse{}, nil
	}
}
func TestRPCErrors(t *testing.T) {

	cfg := config.Load()
	cfg.GRPCAddr = "127.0.0.1:1"
	c, e := rpcdelivery.NewClient(cfg)
	if e != nil {
		t.Fatal(e)
	}
	c.Timeout = 30 * time.Millisecond
	_, e = c.Get(context.Background(), 1, domain.User{ID: 1})
	c.Close()
	var de *domain.Error
	if !errors.As(e, &de) || de.Code != "unavailable" {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	srv := grpc.NewServer()
	pb.RegisterDeliveryServiceServer(srv, slowServer{})
	go srv.Serve(l)
	defer srv.Stop()
	cfg.GRPCAddr = l.Addr().String()
	c, e = rpcdelivery.NewClient(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.Timeout = 20 * time.Millisecond
	_, e = c.Get(context.Background(), 1, domain.User{ID: 1})
	if !errors.As(e, &de) || de.Code != "deadline" {
		t.Fatal(e)
	}
}
func TestHTTPServiceUnavailable(t *testing.T) {

	_, p := setup(t)
	cfg := config.Load()
	cfg.JWTSecret = "test-unavailable-longer-than-thirty-two-characters"
	cfg.GRPCAddr = "127.0.0.1:1"
	cfg.TasksMode = "disabled"
	h, closeFn, e := app.New(context.Background(), p, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer closeFn()
	s := httptest.NewServer(h)
	defer s.Close()
	token, _ := login(t, s, "student")
	code, out := request(t, s, "GET", "/delivery/orders/1", nil, token)
	if code != 503 {
		t.Fatal(code, out)
	}
}
