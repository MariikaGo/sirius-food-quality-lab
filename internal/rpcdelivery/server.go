package rpcdelivery

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"sirius.local/food/internal/config"
	"sirius.local/food/internal/delivery"
	pb "sirius.local/food/internal/deliverypb"
	"sirius.local/food/internal/domain"
	"sirius.local/food/internal/users"
	"strings"
	"time"
)

type Server struct {
	pb.UnimplementedDeliveryServiceServer
	Service  delivery.Service
	Users    users.Store
	Token    string
	Interval time.Duration
}

func NewServer(p *pgxpool.Pool, cfg config.Config) *grpc.Server {
	srv := grpc.NewServer()
	pb.RegisterDeliveryServiceServer(srv, &Server{Service: delivery.Service{Repo: &delivery.Repository{Pool: p}, Secure: true}, Users: &users.Repository{Pool: p}, Token: cfg.ServiceToken, Interval: time.Duration(config.Int("STREAM_INTERVAL_MS", 2000)) * time.Millisecond})
	return srv
}
func rpcError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "Дедлайн операции истёк")
	}
	var de *domain.Error
	if errors.As(e, &de) {
		code := codes.InvalidArgument
		switch de.Code {
		case "not_found":
			code = codes.NotFound
		case "forbidden", "unauthorized":
			code = codes.PermissionDenied
		case "conflict":
			code = codes.FailedPrecondition
		}
		return status.Error(code, de.Message)
	}
	log.Printf("delivery error: %v", e)
	return status.Error(codes.Internal, "Внутренняя ошибка доставки")
}
func (s *Server) actor(ctx context.Context, id int64) (domain.User, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	tokens := md.Get("x-service-token")
	if len(s.Token) < 32 || len(tokens) != 1 || subtle.ConstantTimeCompare([]byte(tokens[0]), []byte(s.Token)) != 1 {
		return domain.User{}, status.Error(codes.Unauthenticated, "Недействительный ключ сервиса")
	}
	u, e := s.Users.ByID(ctx, id)
	if e != nil || u.Blocked {
		return u, status.Error(codes.PermissionDenied, "Пользователь недоступен")
	}
	return u, nil
}
func (s *Server) AssignCourier(ctx context.Context, q *pb.AssignCourierRequest) (*pb.AssignCourierResponse, error) {
	u, e := s.actor(ctx, q.ActorId)
	if e != nil {
		return nil, e
	}
	d, e := s.Service.Assign(ctx, q.OrderId, q.CourierId, &u)
	if e != nil {
		return nil, rpcError(e)
	}
	log.Printf("AssignCourier order=%d courier=%d actor=%d", q.OrderId, q.CourierId, q.ActorId)
	return &pb.AssignCourierResponse{DeliveryId: d.ID, Status: d.Status}, nil
}
func (s *Server) CompleteDelivery(ctx context.Context, q *pb.CompleteDeliveryRequest) (*pb.CompleteDeliveryResponse, error) {
	u, e := s.actor(ctx, q.ActorId)
	if e != nil {
		return nil, e
	}
	if e = s.Service.Complete(ctx, q.DeliveryId, &u); e != nil {
		return nil, rpcError(e)
	}
	log.Printf("CompleteDelivery id=%d actor=%d", q.DeliveryId, q.ActorId)
	return &pb.CompleteDeliveryResponse{Status: "completed"}, nil
}
func (s *Server) GetDelivery(ctx context.Context, q *pb.GetDeliveryRequest) (*pb.GetDeliveryResponse, error) {
	u, e := s.actor(ctx, q.ActorId)
	if e != nil {
		return nil, e
	}
	d, e := s.Service.Get(ctx, q.OrderId, u)
	if e != nil {
		return nil, rpcError(e)
	}
	return &pb.GetDeliveryResponse{Id: d.ID, OrderId: d.OrderID, CourierId: d.CourierID, Status: d.Status}, nil
}
func (s *Server) TrackDelivery(q *pb.TrackDeliveryRequest, stream pb.DeliveryService_TrackDeliveryServer) error {
	ctx := stream.Context()
	u, e := s.actor(ctx, q.ActorId)
	if e != nil {
		return e
	}
	if _, e = s.Service.Get(ctx, q.OrderId, u); e != nil {
		return rpcError(e)
	}
	for i, v := range []string{"in_progress", "on_the_way", "delivered"} {
		if e = stream.Send(&pb.DeliveryStatus{OrderId: q.OrderId, Status: v, Simulated: true}); e != nil {
			return e
		}
		if i < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.Interval):
			}
		}
	}
	return nil
}
func (s *Server) ChatWithCourier(stream pb.DeliveryService_ChatWithCourierServer) error {
	for {
		q, e := stream.Recv()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		u, e := s.actor(stream.Context(), q.ActorId)
		if e != nil {
			return e
		}
		if _, e = s.Service.Get(stream.Context(), q.OrderId, u); e != nil {
			return rpcError(e)
		}
		if strings.TrimSpace(q.Text) == "" || len([]rune(q.Text)) > 2000 {
			return status.Error(codes.InvalidArgument, "Нужно 1–2000 символов")
		}
		if e = stream.Send(&pb.ChatMessage{OrderId: q.OrderId, ActorId: q.ActorId, Sender: "courier-demo", Text: "Курьер (учебный автоответ): " + q.Text}); e != nil {
			return e
		}
	}
}
