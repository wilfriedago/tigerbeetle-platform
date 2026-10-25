// Package amqp keeps a connection to RabbitMQ open. amqp091-go does not reconnect (research (c)):
// a dropped connection stays dropped. Supervisor dials, runs a session on the connection until it
// closes, and dials again with capped backoff, so a broker restart costs a pause, not a process
// restart.
package amqp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Session uses an open connection until it returns. It should return when ctx ends or when the
// connection closes; Run then redials unless ctx has ended.
type Session func(ctx context.Context, conn *amqp091.Connection) error

// Supervisor owns one connection at a time.
type Supervisor struct {
	url    string
	name   string
	logger *slog.Logger
	// minBackoff and maxBackoff bound the pause between two dials.
	minBackoff, maxBackoff time.Duration

	mu   sync.Mutex
	conn *amqp091.Connection
}

// New returns a supervisor for url. name shows in the RabbitMQ management UI.
func New(url, name string, logger *slog.Logger) *Supervisor {
	return &Supervisor{url: url, name: name, logger: logger, minBackoff: 100 * time.Millisecond, maxBackoff: 5 * time.Second}
}

// Run dials and runs session until ctx ends, redialling whenever the connection drops. It returns
// nil once ctx has ended.
func (s *Supervisor) Run(ctx context.Context, session Session) error {
	backoff := s.minBackoff
	for {
		conn, err := amqp091.DialConfig(s.url, amqp091.Config{
			Properties: amqp091.Table{"connection_name": s.name},
			Dial:       amqp091.DefaultDial(10 * time.Second),
		})
		if err == nil {
			backoff = s.minBackoff
			s.set(conn)
			err = session(ctx, conn)
			s.set(nil)
			if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, amqp091.ErrClosed) {
				s.logger.WarnContext(ctx, "amqp close", slog.Any("error", closeErr))
			}
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		// The URL holds the password, so only the error is logged.
		s.logger.WarnContext(ctx, "amqp connection lost, redialling", slog.Any("error", err), slog.Duration("in", backoff))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, s.maxBackoff)
	}
}

func (s *Supervisor) set(conn *amqp091.Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn
}

// Connection returns the open connection, or nil while dialling.
func (s *Supervisor) Connection() *amqp091.Connection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}

// ErrNotConnected means the supervisor has no open connection right now.
var ErrNotConnected = errors.New("amqp: not connected")

// Check runs probe on a fresh channel of the open connection, for readiness.
func (s *Supervisor) Check(ctx context.Context, probe func(ch *amqp091.Channel) error) error {
	conn := s.Connection()
	if conn == nil || conn.IsClosed() {
		return ErrNotConnected
	}
	done := make(chan error, 1)
	go func() {
		ch, err := conn.Channel()
		if err != nil {
			done <- fmt.Errorf("amqp: open channel: %w", err)
			return
		}
		defer func() { _ = ch.Close() }()
		done <- probe(ch)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
