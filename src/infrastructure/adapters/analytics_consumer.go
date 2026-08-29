package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
	usecases "gitlab.com/URL-shortener4224128/analytics-repository/src/application/use_cases"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/core/models"
	"gitlab.com/URL-shortener4224128/analytics-repository/src/infrastructure/gotel"
	"go.opentelemetry.io/otel"
)

type AnalyticsConsumer struct {
	saveAnalyticsUseCase *usecases.SaveAnalyticsUseCase
	queue amqp091.Queue
	dlq amqp091.Queue
	ch *amqp091.Channel
	telemetry gotel.TelemetryProvider
}

func dlq(ctx context.Context, dlq amqp091.Queue, channel *amqp091.Channel, dto *models.Analytic) error {
	msg, err := json.Marshal(dto)
	
	fmt.Println("serializer")
	if err != nil {
		return err
	}

	err = channel.PublishWithContext(ctx,
		"",
		dlq.Name,
		false,
		false,
		amqp091.Publishing{
			ContentType: "text/plain",
			Body:        []byte(msg),
		},
	)

	fmt.Println("publish")
	if err != nil {
		return err
	}

	fmt.Println("finished")
	return nil
}

func NewAnalyticsConsumer(
	saveAnalyticsUseCase *usecases.SaveAnalyticsUseCase,
	ch *amqp091.Channel,
	telemetry gotel.TelemetryProvider,
) (*AnalyticsConsumer, error) {
	q, err := ch.QueueDeclare(
		"visits",
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return nil, err
	}

	dlq, err := ch.QueueDeclare(
		"visits_dlq",
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return nil, err
	}

	return &AnalyticsConsumer{
		saveAnalyticsUseCase: saveAnalyticsUseCase,
		dlq: dlq,
		queue: q,
		ch: ch,
		telemetry: telemetry,
	}, nil
}

type amqpCarrier struct {
	headers amqp091.Table
}

func (c amqpCarrier) Get(key string) string {
	if val, ok := c.headers[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (c amqpCarrier) Set(key string, val string) {
	if c.headers == nil {
		c.headers = make(amqp091.Table)
	}
	c.headers[key] = val
}

func (c amqpCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

func (c *AnalyticsConsumer) Consume(ctx context.Context) (error) {
	msgs, err := c.ch.Consume(
    c.queue.Name,
		"",
    false,
    false,
    false,
    false,
    nil,
	)

	if err != nil {
		return err
	}

	go func() {
    for d := range msgs {

			if d.Headers == nil {
					d.Headers = amqp091.Table{}
			}

			extractedCtx := otel.GetTextMapPropagator().Extract(context.Background(), amqpCarrier{headers: d.Headers})

			ctx, span := c.telemetry.TraceStart(extractedCtx, "AnalyticsConsumer.ProcessMessage")
			defer span.End()

			var analytic *models.Analytic

			if err := json.Unmarshal(d.Body, &analytic); err != nil {
				msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := dlq(msgCtx, c.dlq, c.ch, analytic)
				if err != nil {
					log.Printf("Error saving in dlq: %s", err.Error())
				}
				cancel()

				d.Nack(false, false)
				continue
			}

			maxRetries := 3
			retryDelay := 10 * time.Second
			var err error

			for i := 1; i <= maxRetries; i++ {
				msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

				err = c.saveAnalyticsUseCase.Execute(msgCtx, analytic)
				cancel()

				if err == nil {
						break
				}

				log.Printf("Error saving analytic: %s, tried: %d", err.Error(), i)

				if i < maxRetries {
						time.Sleep(retryDelay)
						retryDelay *= 2
				}
			}

			if err != nil {
				msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := dlq(msgCtx, c.dlq, c.ch, analytic)
				if err != nil {
					log.Printf("Error saving in dlq: %s", err.Error())
				}
				cancel()

				d.Nack(false, false)
				continue
			}

			d.Ack(false)
    }
	}()

	return nil;
}
