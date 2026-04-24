package generator

import pb "github.com/online-cinema/producer/pb"

type sessionState int

const (
	stateIdle sessionState = iota
	stateStarted
	statePaused
	stateResumed
	stateFinished
	statePendingLike
)

type userSession struct {
	SessionID  string
	MovieID    string
	DeviceType pb.DeviceType
	Progress   int32
	State      sessionState
}
