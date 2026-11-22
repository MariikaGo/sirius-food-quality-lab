CREATE TABLE event_outbox(id TEXT PRIMARY KEY,routing_key TEXT NOT NULL,payload JSONB NOT NULL,published_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE processed_events(consumer TEXT NOT NULL,event_id TEXT NOT NULL,processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(consumer,event_id));
CREATE TABLE kitchen_orders(order_id BIGINT PRIMARY KEY REFERENCES orders(id),status TEXT NOT NULL);
