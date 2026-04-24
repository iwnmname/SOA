package generator

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/online-cinema/producer/internal/kafka"
	pb "github.com/online-cinema/producer/pb"
)

var devices = []pb.DeviceType{
	pb.DeviceType_MOBILE,
	pb.DeviceType_DESKTOP,
	pb.DeviceType_TV,
	pb.DeviceType_TABLET,
}

type Generator struct {
	producer  *kafka.Producer
	logger    *zap.Logger
	numUsers  int
	numMovies int
	rateMs    int
	users     []string
	movies    []string
	sessions  map[string]*userSession
	rng       *rand.Rand
	count     atomic.Int64
}

func New(producer *kafka.Producer, logger *zap.Logger, numUsers, numMovies, rateMs int) *Generator {
	users := make([]string, numUsers)
	for i := range users {
		users[i] = fmt.Sprintf("user-%04d", i+1)
	}

	movies := make([]string, numMovies)
	for i := range movies {
		movies[i] = fmt.Sprintf("movie-%04d", i+1)
	}

	return &Generator{
		producer:  producer,
		logger:    logger,
		numUsers:  numUsers,
		numMovies: numMovies,
		rateMs:    rateMs,
		users:     users,
		movies:    movies,
		sessions:  make(map[string]*userSession),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (g *Generator) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration(g.rateMs) * time.Millisecond)
	defer ticker.Stop()

	g.logger.Info("generator started",
		zap.Int("users", g.numUsers),
		zap.Int("movies", g.numMovies),
		zap.Int("rate_ms", g.rateMs),
	)

	for {
		select {
		case <-ctx.Done():
			g.logger.Info("generator stopped", zap.Int64("total_events", g.count.Load()))
			return nil
		case <-ticker.C:
			event := g.next()
			if err := g.producer.Publish(ctx, event); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				g.logger.Error("generator publish failed", zap.Error(err))
				continue
			}
			n := g.count.Add(1)
			if n%100 == 0 {
				g.logger.Info("generator progress", zap.Int64("events_sent", n))
			}
		}
	}
}

func (g *Generator) Count() int64 {
	return g.count.Load()
}

func (g *Generator) next() *pb.MovieEvent {
	userID := g.users[g.rng.Intn(len(g.users))]

	sess, exists := g.sessions[userID]

	if exists && sess.State == statePendingLike {
		delete(g.sessions, userID)
		return g.makeEvent(userID, sess.MovieID, pb.EventType_LIKED, sess.SessionID, sess.Progress, sess.DeviceType)
	}

	if g.rng.Float64() < 0.08 {
		return g.makeEvent(userID, g.randomMovie(), pb.EventType_SEARCHED, uuid.New().String(), 0, devices[g.rng.Intn(len(devices))])
	}

	if !exists {
		return g.startSession(userID)
	}

	return g.continueSession(userID, sess)
}

func (g *Generator) startSession(userID string) *pb.MovieEvent {
	sess := &userSession{
		SessionID:  uuid.New().String(),
		MovieID:    g.randomMovie(),
		DeviceType: devices[g.rng.Intn(len(devices))],
		Progress:   0,
		State:      stateStarted,
	}
	g.sessions[userID] = sess
	return g.makeEvent(userID, sess.MovieID, pb.EventType_VIEW_STARTED, sess.SessionID, 0, sess.DeviceType)
}

func (g *Generator) continueSession(userID string, sess *userSession) *pb.MovieEvent {
	sess.Progress += int32(10 + g.rng.Intn(60))

	switch sess.State {
	case stateStarted, stateResumed:
		return g.handleWatching(userID, sess)
	case statePaused:
		return g.handlePaused(userID, sess)
	default:
		return g.startSession(userID)
	}
}

func (g *Generator) handleWatching(userID string, sess *userSession) *pb.MovieEvent {
	roll := g.rng.Float64()

	if roll < 0.25 {
		sess.State = statePaused
		return g.makeEvent(userID, sess.MovieID, pb.EventType_VIEW_PAUSED, sess.SessionID, sess.Progress, sess.DeviceType)
	}

	if roll < 0.50 {
		return g.finishSession(userID, sess)
	}

	sess.State = stateResumed
	return g.makeEvent(userID, sess.MovieID, pb.EventType_VIEW_RESUMED, sess.SessionID, sess.Progress, sess.DeviceType)
}

func (g *Generator) handlePaused(userID string, sess *userSession) *pb.MovieEvent {
	if g.rng.Float64() < 0.65 {
		sess.State = stateResumed
		return g.makeEvent(userID, sess.MovieID, pb.EventType_VIEW_RESUMED, sess.SessionID, sess.Progress, sess.DeviceType)
	}
	return g.finishSession(userID, sess)
}

func (g *Generator) finishSession(userID string, sess *userSession) *pb.MovieEvent {
	event := g.makeEvent(userID, sess.MovieID, pb.EventType_VIEW_FINISHED, sess.SessionID, sess.Progress, sess.DeviceType)

	if g.rng.Float64() < 0.35 {
		sess.State = statePendingLike
	} else {
		delete(g.sessions, userID)
	}

	return event
}

func (g *Generator) makeEvent(userID, movieID string, eventType pb.EventType, sessionID string, progress int32, deviceType pb.DeviceType) *pb.MovieEvent {
	return &pb.MovieEvent{
		EventId:         uuid.New().String(),
		UserId:          userID,
		MovieId:         movieID,
		EventType:       eventType,
		TimestampMs:     time.Now().UTC().UnixMilli(),
		DeviceType:      deviceType,
		SessionId:       sessionID,
		ProgressSeconds: progress,
	}
}

func (g *Generator) randomMovie() string {
	return g.movies[g.rng.Intn(len(g.movies))]
}
