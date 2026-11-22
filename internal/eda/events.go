package eda

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"sirius.local/food/internal/domain"
)

type Event struct {
	TraceParent   string       `json:"traceparent,omitempty"`
	ID            string       `json:"event_id"`
	Type          string       `json:"type"`
	OrderID       int64        `json:"order_id"`
	UserID        int64        `json:"user_id"`
	Amount        domain.Money `json:"amount"`
	CorrelationID string       `json:"correlation_id"`
}
type correlationKey struct{}

func WithCorrelation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}
func Correlation(ctx context.Context) string { v, _ := ctx.Value(correlationKey{}).(string); return v }
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Record(ctx context.Context, tx pgx.Tx, key string, order int64) error {
	e := Event{ID: ID(), Type: key, OrderID: order, CorrelationID: Correlation(ctx)}
	if e.CorrelationID == "" {
		e.CorrelationID = e.ID
	}
	if err := tx.QueryRow(ctx, "SELECT user_id,total_cents FROM orders WHERE id=$1", order).Scan(&e.UserID, &e.Amount); err != nil {
		return err
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	e.TraceParent = carrier.Get("traceparent")
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO event_outbox(id,routing_key,payload)VALUES($1,$2,$3)", e.ID, key, b)
	return err
}
func logEvent(kind string, e Event, consumer string) {
	b, _ := json.Marshal(map[string]any{"event": kind, "event_id": e.ID, "order_id": e.OrderID, "correlation_id": e.CorrelationID, "consumer": consumer})
	log.Print(string(b))
}
func setup(ch *amqp.Channel) error {
	for _, x := range []string{"sirius_events", "sirius_dead"} {
		if e := ch.ExchangeDeclare(x, "topic", true, false, false, false, nil); e != nil {
			return e
		}
	}
	if _, e := ch.QueueDeclare("sirius.dlq", true, false, false, false, nil); e != nil {
		return e
	}
	if e := ch.QueueBind("sirius.dlq", "#", "sirius_dead", false, nil); e != nil {
		return e
	}
	for name, keys := range map[string][]string{"kitchen": {"order.paid", "order.cancelled"}, "delivery": {"order.paid"}, "compensation": {"delivery.failed"}} {
		q := "sirius." + name
		if _, e := ch.QueueDeclare(q, true, false, false, false, amqp.Table{"x-dead-letter-exchange": "sirius_dead"}); e != nil {
			return e
		}
		for _, key := range keys {
			if e := ch.QueueBind(q, key, "sirius_events", false, nil); e != nil {
				return e
			}
		}
	}
	return ch.Qos(1, 0, false)
}
func send(ctx context.Context, ch *amqp.Channel, confirms <-chan amqp.Confirmation, exchange, key string, m amqp.Publishing) error {
	if e := ch.PublishWithContext(ctx, exchange, key, false, false, m); e != nil {
		return e
	}
	select {
	case c, ok := <-confirms:
		if !ok || !c.Ack {
			return errors.New("publisher confirmation rejected")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func PublishPending(ctx context.Context, p *pgxpool.Pool, ch *amqp.Channel, confirms <-chan amqp.Confirmation) error {
	tx, e := p.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	rows, e := tx.Query(ctx, "SELECT id,routing_key,payload FROM event_outbox WHERE published_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 50")
	if e != nil {
		return e
	}
	type row struct {
		id, key string
		b       []byte
	}
	var pending []row
	for rows.Next() {
		var r row
		if e = rows.Scan(&r.id, &r.key, &r.b); e != nil {
			rows.Close()
			return e
		}
		pending = append(pending, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, r := range pending {
		var ev Event
		if e = json.Unmarshal(r.b, &ev); e != nil {
			return e
		}
		deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
		e = send(deadline, ch, confirms, "sirius_events", r.key, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: r.id, CorrelationId: ev.CorrelationID, Body: r.b})
		cancel()
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE event_outbox SET published_at=now() WHERE id=$1", r.id); e != nil {
			return e
		}
		logEvent("published", ev, "publisher")
	}
	return tx.Commit(ctx)
}
func Process(ctx context.Context, p *pgxpool.Pool, consumer string, e Event) error {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier{"traceparent": e.TraceParent})
	ctx, span := otel.Tracer("sirius.events").Start(ctx, "consume "+consumer)
	defer span.End()
	span.SetAttributes(attribute.String("correlation_id", e.CorrelationID), attribute.String("event_id", e.ID))
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "INSERT INTO processed_events(consumer,event_id)VALUES($1,$2)ON CONFLICT DO NOTHING", consumer, e.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		logEvent("duplicate_skipped", e, consumer)
		return nil
	}
	var owner int64
	var total domain.Money
	var state string
	if err = tx.QueryRow(ctx, "SELECT user_id,total_cents,status FROM orders WHERE id=$1 FOR UPDATE", e.OrderID).Scan(&owner, &total, &state); err != nil {
		return err
	}
	if owner != e.UserID || total != e.Amount {
		return errors.New("event does not match authoritative order")
	}
	ctx = WithCorrelation(ctx, e.CorrelationID)
	switch consumer {
	case "kitchen":
		status := "queued"
		if state == "cancelled" || e.Type == "order.cancelled" {
			status = "cancelled"
		}
		_, err = tx.Exec(ctx, "INSERT INTO kitchen_orders(order_id,status)VALUES($1,$2)ON CONFLICT(order_id)DO UPDATE SET status=EXCLUDED.status", e.OrderID, status)
	case "delivery":
		if state != "paid" {
			break
		}
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM deliveries WHERE order_id=$1)", e.OrderID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			break
		}
		var courier int64
		err = tx.QueryRow(ctx, "SELECT id FROM users u WHERE role='courier' AND NOT is_blocked AND NOT EXISTS(SELECT 1 FROM deliveries d WHERE d.courier_id=u.id AND d.status='in_progress')ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1").Scan(&courier)
		if errors.Is(err, pgx.ErrNoRows) {
			err = Record(ctx, tx, "delivery.failed", e.OrderID)
		} else if err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO deliveries(order_id,courier_id)VALUES($1,$2)", e.OrderID, courier)
		}
	case "compensation":
		if state == "paid" {
			if _, err = tx.Exec(ctx, "UPDATE users SET balance_cents=balance_cents+$2 WHERE id=$1", owner, total); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "UPDATE orders SET status='cancelled' WHERE id=$1", e.OrderID); err != nil {
				return err
			}
			err = Record(ctx, tx, "order.cancelled", e.OrderID)
		}
	default:
		return fmt.Errorf("unknown consumer %s", consumer)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	logEvent("processed", e, consumer)
	return nil
}
func session(ctx context.Context, p *pgxpool.Pool, mode string) error {
	conn, e := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if e != nil {
		return e
	}
	defer conn.Close()
	ch, e := conn.Channel()
	if e != nil {
		return e
	}
	defer ch.Close()
	if e = setup(ch); e != nil {
		return e
	}
	if e = ch.Confirm(false); e != nil {
		return e
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	if mode == "publisher" {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if e = PublishPending(ctx, p, ch, confirms); e != nil {
				return e
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}
	messages, e := ch.Consume("sirius."+mode, "", false, false, false, false, nil)
	if e != nil {
		return e
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-messages:
			if !ok {
				return errors.New("consumer channel closed")
			}
			var ev Event
			if json.Unmarshal(d.Body, &ev) != nil || ev.ID == "" || ev.OrderID <= 0 || ev.UserID <= 0 || ev.CorrelationID == "" || !validType(mode, ev.Type) {
				_ = d.Nack(false, false)
				continue
			}
			if e = Process(ctx, p, mode, ev); e == nil {
				_ = d.Ack(false)
				continue
			}
			attempt := int32(0)
			if n, ok := d.Headers["attempt"].(int32); ok {
				attempt = n
			}
			if attempt >= 2 {
				_ = d.Nack(false, false)
				continue
			}
			deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
			e = send(deadline, ch, confirms, "", "sirius."+mode, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: ev.ID, CorrelationId: ev.CorrelationID, Body: d.Body, Headers: amqp.Table{"attempt": attempt + 1}})
			cancel()
			if e != nil {
				_ = d.Nack(false, true)
				return e
			}
			_ = d.Ack(false)
		}
	}
}
func validType(mode, key string) bool {
	switch mode {
	case "kitchen":
		return key == "order.paid" || key == "order.cancelled"
	case "delivery":
		return key == "order.paid"
	case "compensation":
		return key == "delivery.failed"
	}
	return false
}
func Run(ctx context.Context, p *pgxpool.Pool, mode string) {
	for ctx.Err() == nil {
		if e := session(ctx, p, mode); e != nil && ctx.Err() == nil {
			log.Printf("%s reconnect: %v", mode, e)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
