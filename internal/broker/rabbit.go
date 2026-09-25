package broker

import (
	"context"
	"encoding/json"

	"github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"goph-profile/internal/domain"
	"goph-profile/internal/observability"
)

const Queue = "avatar.processing"

type Rabbit struct {
	conn *amqp091.Connection
	ch   *amqp091.Channel
}

type tableCarrier amqp091.Table

func (c tableCarrier) Get(key string) string {
	if v, ok := c[key].(string); ok {
		return v
	}
	return ""
}
func (c tableCarrier) Set(key, value string) { c[key] = value }
func (c tableCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

func ExtractContext(ctx context.Context, headers amqp091.Table) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, tableCarrier(headers))
}

func (r *Rabbit) Healthy() bool { return !r.conn.IsClosed() }

func NewRabbit(url string) (*Rabbit, error) {
	c, e := amqp091.Dial(url)
	if e != nil {
		return nil, e
	}
	ch, e := c.Channel()
	if e != nil {
		_ = c.Close()
		return nil, e
	}
	if _, e = ch.QueueDeclare(Queue, true, false, false, false, nil); e != nil {
		_ = ch.Close()
		_ = c.Close()
		return nil, e
	}
	return &Rabbit{c, ch}, nil
}
func (r *Rabbit) Close() { _ = r.ch.Close(); _ = r.conn.Close() }
func (r *Rabbit) Publish(ctx context.Context, event domain.ProcessEvent) error {
	ctx, span := otel.Tracer("goph-profile/rabbitmq").Start(ctx, "rabbitmq.publish", trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(attribute.String("messaging.system", "rabbitmq"), attribute.String("messaging.destination.name", Queue), attribute.String("messaging.message.id", event.MessageID)))
	defer span.End()
	b, e := json.Marshal(event)
	if e != nil {
		return e
	}
	headers := amqp091.Table{}
	otel.GetTextMapPropagator().Inject(ctx, tableCarrier(headers))
	e = r.ch.PublishWithContext(ctx, "", Queue, false, false, amqp091.Publishing{ContentType: "application/json", DeliveryMode: amqp091.Persistent, MessageId: event.MessageID, Headers: headers, Body: b})
	status := "success"
	if e != nil {
		status = "error"
		span.RecordError(e)
	}
	observability.QueuePublished.WithLabelValues(status).Inc()
	if q, inspectErr := r.ch.QueueInspect(Queue); inspectErr == nil {
		observability.QueueDepth.Set(float64(q.Messages))
	}
	return e
}
func (r *Rabbit) Consume() (<-chan amqp091.Delivery, error) {
	if e := r.ch.Qos(1, 0, false); e != nil {
		return nil, e
	}
	return r.ch.Consume(Queue, "", false, false, false, false, nil)
}

var _ propagation.TextMapCarrier = tableCarrier{}
