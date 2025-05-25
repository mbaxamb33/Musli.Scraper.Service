// internal/queue/rabbitmq.go
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type RabbitMQManager struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	queueName string
	dlqName   string
	exchange  string
	logger    *zap.Logger
}

type JobMessage struct {
	JobID     string    `json:"job_id"`
	URL       string    `json:"url"`
	Priority  int       `json:"priority,omitempty"`
	Retry     int       `json:"retry,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func NewRabbitMQManager(url, queueName, dlqName, exchange string, logger *zap.Logger) (*RabbitMQManager, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	manager := &RabbitMQManager{
		conn:      conn,
		channel:   ch,
		queueName: queueName,
		dlqName:   dlqName,
		exchange:  exchange,
		logger:    logger,
	}

	if err := manager.setupQueuesAndExchange(); err != nil {
		manager.Close()
		return nil, fmt.Errorf("failed to setup queues: %w", err)
	}

	return manager, nil
}

func (r *RabbitMQManager) setupQueuesAndExchange() error {
	// Declare exchange
	err := r.channel.ExchangeDeclare(
		r.exchange, // name
		"direct",   // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare dead letter queue
	_, err = r.channel.QueueDeclare(
		r.dlqName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Bind DLQ to exchange
	err = r.channel.QueueBind(
		r.dlqName,  // queue name
		"failed",   // routing key
		r.exchange, // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	// Declare main queue with DLX
	_, err = r.channel.QueueDeclare(
		r.queueName, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    r.exchange,
			"x-dead-letter-routing-key": "failed",
			"x-message-ttl":             300000, // 5 minutes
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare main queue: %w", err)
	}

	// Bind main queue to exchange
	err = r.channel.QueueBind(
		r.queueName, // queue name
		"scraping",  // routing key
		r.exchange,  // exchange
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind main queue: %w", err)
	}

	// Set QoS to process one message at a time
	err = r.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	return nil
}

func (r *RabbitMQManager) PublishJob(ctx context.Context, job JobMessage) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return r.channel.PublishWithContext(
		ctx,
		r.exchange, // exchange
		"scraping", // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // Make message persistent
			Priority:     uint8(job.Priority),
			Timestamp:    time.Now(),
		},
	)
}

func (r *RabbitMQManager) ConsumeJobs(ctx context.Context, handler func(JobMessage) error) error {
	msgs, err := r.channel.Consume(
		r.queueName, // queue
		"",          // consumer
		false,       // auto-ack (we'll ack manually)
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("channel closed")
			}

			var job JobMessage
			if err := json.Unmarshal(d.Body, &job); err != nil {
				r.logger.Error("Failed to unmarshal job", zap.Error(err))
				d.Nack(false, false) // Don't requeue malformed messages
				continue
			}

			r.logger.Debug("Processing job", zap.String("job_id", job.JobID))

			if err := handler(job); err != nil {
				r.logger.Error("Job processing failed",
					zap.String("job_id", job.JobID),
					zap.Error(err))

				// Check retry count
				if job.Retry < 3 {
					// Requeue with increased retry count
					job.Retry++
					if err := r.PublishJob(ctx, job); err != nil {
						r.logger.Error("Failed to requeue job", zap.Error(err))
					}
				}

				d.Nack(false, false) // Don't requeue, we handled retry logic
			} else {
				d.Ack(false) // Acknowledge successful processing
			}
		}
	}
}

func (r *RabbitMQManager) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *RabbitMQManager) GetQueueInfo() (int, error) {
	queue, err := r.channel.QueueInspect(r.queueName)
	if err != nil {
		return 0, err
	}
	return queue.Messages, nil
}
