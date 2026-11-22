package rpcdelivery

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"sirius.local/food/internal/config"
	pb "sirius.local/food/internal/deliverypb"
	"sirius.local/food/internal/domain"
	"time"
)

type Client struct {
	Conn    *grpc.ClientConn
	Stub    pb.DeliveryServiceClient
	Token   string
	Timeout time.Duration
}

func NewClient(cfg config.Config) (*Client, error) {
	conn, e := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if e != nil {
		return nil, e
	}
	return &Client{conn, pb.NewDeliveryServiceClient(conn), cfg.ServiceToken, 5 * time.Second}, nil
}
func (c *Client) Close() { c.Conn.Close() }
func (c *Client) ctx(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-service-token", c.Token)
	return context.WithTimeout(ctx, d)
}
func translate(e error) error {
	if e == nil {
		return nil
	}
	code := "unavailable"
	switch status.Code(e) {
	case codes.NotFound:
		code = "not_found"
	case codes.PermissionDenied, codes.Unauthenticated:
		code = "forbidden"
	case codes.InvalidArgument, codes.FailedPrecondition:
		code = "invalid"
	case codes.DeadlineExceeded:
		code = "deadline"
	}
	message := status.Convert(e).Message()
	if code == "unavailable" {
		message = "Сервис доставки недоступен"
	}
	return domain.Fail(code, message)
}
func (c *Client) Assign(ctx context.Context, order, courier int64, actor *domain.User) (domain.Delivery, error) {
	if actor == nil {
		return domain.Delivery{}, domain.Fail("unauthorized", "Нужен пользователь")
	}
	ctx, cancel := c.ctx(ctx, c.Timeout)
	defer cancel()
	v, e := c.Stub.AssignCourier(ctx, &pb.AssignCourierRequest{OrderId: order, CourierId: courier, ActorId: actor.ID})
	if e != nil {
		return domain.Delivery{}, translate(e)
	}
	return domain.Delivery{ID: v.DeliveryId, OrderID: order, CourierID: courier, Status: v.Status}, nil
}
func (c *Client) Get(ctx context.Context, order int64, actor domain.User) (domain.Delivery, error) {
	ctx, cancel := c.ctx(ctx, c.Timeout)
	defer cancel()
	v, e := c.Stub.GetDelivery(ctx, &pb.GetDeliveryRequest{OrderId: order, ActorId: actor.ID})
	if e != nil {
		return domain.Delivery{}, translate(e)
	}
	return domain.Delivery{ID: v.Id, OrderID: v.OrderId, CourierID: v.CourierId, Status: v.Status}, nil
}
func (c *Client) Complete(ctx context.Context, id int64, actor *domain.User) error {
	if actor == nil {
		return domain.Fail("unauthorized", "Нужен пользователь")
	}
	ctx, cancel := c.ctx(ctx, c.Timeout)
	defer cancel()
	_, e := c.Stub.CompleteDelivery(ctx, &pb.CompleteDeliveryRequest{DeliveryId: id, ActorId: actor.ID})
	return translate(e)
}
func (c *Client) Track(ctx context.Context, order int64, actor domain.User) ([]*pb.DeliveryStatus, error) {
	ctx, cancel := c.ctx(ctx, 15*time.Second)
	defer cancel()
	stream, e := c.Stub.TrackDelivery(ctx, &pb.TrackDeliveryRequest{OrderId: order, ActorId: actor.ID})
	if e != nil {
		return nil, translate(e)
	}
	out := []*pb.DeliveryStatus{}
	for {
		v, e := stream.Recv()
		if e == io.EOF {
			return out, nil
		}
		if e != nil {
			return out, translate(e)
		}
		out = append(out, v)
	}
}
func (c *Client) Chat(ctx context.Context, order int64, actor domain.User, texts []string) ([]string, error) {
	ctx, cancel := c.ctx(ctx, 30*time.Second)
	defer cancel()
	stream, e := c.Stub.ChatWithCourier(ctx)
	if e != nil {
		return nil, translate(e)
	}
	out := []string{}
	for _, text := range texts {
		if e = stream.Send(&pb.ChatMessage{OrderId: order, ActorId: actor.ID, Text: text}); e != nil {
			return out, translate(e)
		}
		v, e := stream.Recv()
		if e != nil {
			return out, translate(e)
		}
		out = append(out, v.Text)
	}
	if e = stream.CloseSend(); e != nil {
		return out, e
	}
	_, e = stream.Recv()
	if e != io.EOF {
		return out, translate(e)
	}
	return out, nil
}
