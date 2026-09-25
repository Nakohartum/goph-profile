package broker

import (
	"context"
	"encoding/json"

	"github.com/rabbitmq/amqp091-go"

	"goph-profile/internal/domain"
)

const Queue = "avatar.processing"

type Rabbit struct {
	conn *amqp091.Connection
	ch   *amqp091.Channel
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
	b, e := json.Marshal(event)
	if e != nil {
		return e
	}
	return r.ch.PublishWithContext(ctx, "", Queue, false, false, amqp091.Publishing{ContentType: "application/json", DeliveryMode: amqp091.Persistent, MessageId: event.MessageID, Body: b})
}
func (r *Rabbit) Consume() (<-chan amqp091.Delivery, error) {
	if e := r.ch.Qos(1, 0, false); e != nil {
		return nil, e
	}
	return r.ch.Consume(Queue, "", false, false, false, false, nil)
}
