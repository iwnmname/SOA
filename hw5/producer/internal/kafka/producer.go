package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	pb "github.com/online-cinema/producer/pb"
)

type Config struct {
	Brokers        []string
	Topic          string
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type Producer struct {
	writer         *kafkago.Writer
	brokers        []string
	topic          string
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	logger         *zap.Logger
}

func NewProducer(cfg Config, logger *zap.Logger) (*Producer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers are required")
	}
	if strings.TrimSpace(cfg.Topic) == "" {
		return nil, errors.New("kafka topic is required")
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 100 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 2 * time.Second
	}

	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireAll,
		Async:        false,
	}

	return &Producer{
		writer:         writer,
		brokers:        cfg.Brokers,
		topic:          cfg.Topic,
		maxRetries:     cfg.MaxRetries,
		initialBackoff: cfg.InitialBackoff,
		maxBackoff:     cfg.MaxBackoff,
		logger:         logger,
	}, nil
}

func (p *Producer) Publish(ctx context.Context, event *pb.MovieEvent) error {
	if err := validateEvent(event); err != nil {
		return fmt.Errorf("validate event: %w", err)
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal protobuf event: %w", err)
	}

	key := []byte(event.UserId)
	if len(key) == 0 {
		key = []byte(event.SessionId)
	}

	msg := kafkago.Message{
		Key:   key,
		Value: payload,
		Time:  time.Now().UTC(),
	}

	backoff := p.initialBackoff
	var lastErr error

	for attempt := 1; attempt <= p.maxRetries; attempt++ {
		if err := p.writer.WriteMessages(ctx, msg); err == nil {
			p.logger.Info("event published",
				zap.String("event_id", event.EventId),
				zap.String("event_type", event.EventType.String()),
				zap.Int64("timestamp_ms", event.TimestampMs),
			)
			return nil
		} else {
			lastErr = err
		}

		if attempt == p.maxRetries {
			break
		}

		p.logger.Warn("publish retry",
			zap.String("event_id", event.EventId),
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(lastErr),
		)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		backoff *= 2
		if backoff > p.maxBackoff {
			backoff = p.maxBackoff
		}
	}

	return fmt.Errorf("publish to kafka topic %q after %d attempts: %w", p.topic, p.maxRetries, lastErr)
}

func (p *Producer) Health(ctx context.Context) error {
	dialer := &kafkago.Dialer{Timeout: 2 * time.Second}
	for _, broker := range p.brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err == nil {
			_ = conn.Close()
			return nil
		}
	}
	return fmt.Errorf("kafka unavailable: brokers=%v", p.brokers)
}

func (p *Producer) Close() error {
	if p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func validateEvent(event *pb.MovieEvent) error {
	if event == nil {
		return errors.New("event is nil")
	}
	if event.EventId == "" {
		return errors.New("event_id is required")
	}
	if _, err := uuid.Parse(event.EventId); err != nil {
		return errors.New("event_id must be valid UUID")
	}
	if strings.TrimSpace(event.UserId) == "" {
		return errors.New("user_id is required")
	}
	if strings.TrimSpace(event.MovieId) == "" {
		return errors.New("movie_id is required")
	}
	if event.EventType == pb.EventType_EVENT_TYPE_UNSPECIFIED {
		return errors.New("event_type is required")
	}
	if event.TimestampMs <= 0 {
		return errors.New("timestamp must be > 0")
	}
	if event.DeviceType == pb.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return errors.New("device_type is required")
	}
	if strings.TrimSpace(event.SessionId) == "" {
		return errors.New("session_id is required")
	}
	if event.ProgressSeconds < 0 {
		return errors.New("progress_seconds must be >= 0")
	}
	return nil
}
