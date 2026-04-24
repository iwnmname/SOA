package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
	pb "github.com/online-cinema/producer/pb"
)

var eventTypeMap = map[string]pb.EventType{
	"VIEW_STARTED":  pb.EventType_VIEW_STARTED,
	"VIEW_FINISHED": pb.EventType_VIEW_FINISHED,
	"VIEW_PAUSED":   pb.EventType_VIEW_PAUSED,
	"VIEW_RESUMED":  pb.EventType_VIEW_RESUMED,
	"LIKED":         pb.EventType_LIKED,
	"SEARCHED":      pb.EventType_SEARCHED,
}

var deviceTypeMap = map[string]pb.DeviceType{
	"MOBILE":  pb.DeviceType_MOBILE,
	"DESKTOP": pb.DeviceType_DESKTOP,
	"TV":      pb.DeviceType_TV,
	"TABLET":  pb.DeviceType_TABLET,
}

var (
	ErrMissingEventID    = errors.New("event_id is required")
	ErrInvalidEventID    = errors.New("event_id must be valid UUID")
	ErrMissingUserID     = errors.New("user_id is required")
	ErrMissingMovieID    = errors.New("movie_id is required")
	ErrInvalidEventType  = errors.New("invalid event_type")
	ErrInvalidDeviceType = errors.New("invalid device_type")
	ErrMissingSessionID  = errors.New("session_id is required")
	ErrNegativeProgress  = errors.New("progress_seconds must be >= 0")
)

type EventRequest struct {
	EventID         string `json:"event_id" binding:"required"`
	UserID          string `json:"user_id" binding:"required"`
	MovieID         string `json:"movie_id" binding:"required"`
	EventType       string `json:"event_type" binding:"required"`
	Timestamp       string `json:"timestamp"`
	DeviceType      string `json:"device_type" binding:"required"`
	SessionID       string `json:"session_id" binding:"required"`
	ProgressSeconds int32  `json:"progress_seconds"`
}

type EventResponse struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

type BatchRequest struct {
	Events []EventRequest `json:"events" binding:"required,dive"`
}

type BatchResponse struct {
	Total     int             `json:"total"`
	Published int             `json:"published"`
	Failed    int             `json:"failed"`
	Results   []EventResponse `json:"results"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func (e *EventRequest) Validate() error {
	if e.EventID == "" {
		return ErrMissingEventID
	}
	if _, err := uuid.Parse(e.EventID); err != nil {
		return ErrInvalidEventID
	}
	if e.UserID == "" {
		return ErrMissingUserID
	}
	if e.MovieID == "" {
		return ErrMissingMovieID
	}
	if _, ok := eventTypeMap[e.EventType]; !ok {
		return ErrInvalidEventType
	}
	if _, ok := deviceTypeMap[e.DeviceType]; !ok {
		return ErrInvalidDeviceType
	}
	if e.SessionID == "" {
		return ErrMissingSessionID
	}
	if e.ProgressSeconds < 0 {
		return ErrNegativeProgress
	}
	return nil
}

func (e *EventRequest) ToProto() *pb.MovieEvent {
	var ts int64
	if e.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			ts = t.UnixMilli()
		}
	}
	if ts == 0 {
		ts = time.Now().UTC().UnixMilli()
	}

	return &pb.MovieEvent{
		EventId:         e.EventID,
		UserId:          e.UserID,
		MovieId:         e.MovieID,
		EventType:       eventTypeMap[e.EventType],
		TimestampMs:     ts,
		DeviceType:      deviceTypeMap[e.DeviceType],
		SessionId:       e.SessionID,
		ProgressSeconds: e.ProgressSeconds,
	}
}

func EventTypeToString(et pb.EventType) string {
	for k, v := range eventTypeMap {
		if v == et {
			return k
		}
	}
	return "UNKNOWN"
}

func DeviceTypeToString(dt pb.DeviceType) string {
	for k, v := range deviceTypeMap {
		if v == dt {
			return k
		}
	}
	return "UNKNOWN"
}
