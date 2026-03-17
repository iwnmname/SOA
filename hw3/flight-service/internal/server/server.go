package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"hw3/flight-service/internal/model"
	"hw3/flight-service/internal/repository"
	flightpb "hw3/gen/flight"
)

type FlightServer struct {
	flightpb.UnimplementedFlightServiceServer
	repo     *repository.FlightRepository
	redis    *redis.Client
	cacheTTL time.Duration
}

func NewFlightServer(repo *repository.FlightRepository, redisClient *redis.Client, cacheTTL time.Duration) *FlightServer {
	return &FlightServer{repo: repo, redis: redisClient, cacheTTL: cacheTTL}
}

func (s *FlightServer) SearchFlights(ctx context.Context, req *flightpb.SearchFlightsRequest) (*flightpb.SearchFlightsResponse, error) {
	if req.Origin == "" || req.Destination == "" {
		return nil, status.Error(codes.InvalidArgument, "origin and destination are required")
	}

	var date *time.Time
	if req.Date != nil {
		t := req.Date.AsTime()
		date = &t
	}

	searchDate := "any"
	if date != nil {
		searchDate = date.Format("2006-01-02")
	}

	searchKey := searchCacheKey(req.Origin, req.Destination, searchDate)

	if cachedFlights, ok := s.getCachedSearch(ctx, searchKey); ok {
		resp := &flightpb.SearchFlightsResponse{}
		for i := range cachedFlights {
			f := cachedFlights[i]
			resp.Flights = append(resp.Flights, toProtoFlight(&f))
		}
		return resp, nil
	}

	flights, err := s.repo.Search(ctx, req.Origin, req.Destination, date)
	if err != nil {
		log.Printf("search flights error: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	s.setCachedSearch(ctx, searchKey, flights)

	resp := &flightpb.SearchFlightsResponse{}
	for _, f := range flights {
		resp.Flights = append(resp.Flights, toProtoFlight(f))
	}
	return resp, nil
}

func (s *FlightServer) GetFlight(ctx context.Context, req *flightpb.GetFlightRequest) (*flightpb.GetFlightResponse, error) {
	if req.FlightId == "" {
		return nil, status.Error(codes.InvalidArgument, "flight_id is required")
	}

	flightKey := flightCacheKey(req.FlightId)
	if cachedFlight, ok := s.getCachedFlight(ctx, flightKey); ok {
		return &flightpb.GetFlightResponse{Flight: toProtoFlight(cachedFlight)}, nil
	}

	flight, err := s.repo.GetByID(ctx, req.FlightId)
	if err != nil {
		log.Printf("get flight error: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	if flight == nil {
		return nil, status.Error(codes.NotFound, "flight not found")
	}

	s.setCachedFlight(ctx, flightKey, flight)

	return &flightpb.GetFlightResponse{Flight: toProtoFlight(flight)}, nil
}

func (s *FlightServer) ReserveSeats(ctx context.Context, req *flightpb.ReserveSeatsRequest) (*flightpb.ReserveSeatsResponse, error) {
	if req.FlightId == "" || req.BookingId == "" {
		return nil, status.Error(codes.InvalidArgument, "flight_id and booking_id are required")
	}
	if req.SeatCount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "seat_count must be positive")
	}

	reservation, err := s.repo.ReserveSeats(ctx, req.FlightId, req.BookingId, req.SeatCount)
	if err != nil {
		return nil, mapRepoError(err)
	}

	s.invalidateCaches(ctx, reservation.FlightID)

	return &flightpb.ReserveSeatsResponse{Reservation: toProtoReservation(reservation)}, nil
}

func (s *FlightServer) ReleaseReservation(ctx context.Context, req *flightpb.ReleaseReservationRequest) (*flightpb.ReleaseReservationResponse, error) {
	if req.BookingId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}

	reservation, err := s.repo.ReleaseReservation(ctx, req.BookingId)
	if err != nil {
		return nil, mapRepoError(err)
	}

	s.invalidateCaches(ctx, reservation.FlightID)

	return &flightpb.ReleaseReservationResponse{Reservation: toProtoReservation(reservation)}, nil
}

func flightCacheKey(flightID string) string {
	return "flight:" + flightID
}

func searchCacheKey(origin, destination, date string) string {
	return "search:" + origin + ":" + destination + ":" + date
}

func (s *FlightServer) getCachedSearch(ctx context.Context, key string) ([]model.Flight, bool) {
	if s.redis == nil {
		return nil, false
	}

	payload, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		log.Printf("cache miss key=%s", key)
		return nil, false
	}
	if err != nil {
		log.Printf("cache error (get key=%s): %v", key, err)
		return nil, false
	}

	var flights []model.Flight
	if err := json.Unmarshal([]byte(payload), &flights); err != nil {
		log.Printf("cache error (unmarshal key=%s): %v", key, err)
		return nil, false
	}

	log.Printf("cache hit key=%s", key)
	return flights, true
}

func (s *FlightServer) setCachedSearch(ctx context.Context, key string, flights []*model.Flight) {
	if s.redis == nil {
		return
	}

	payload, err := json.Marshal(flights)
	if err != nil {
		log.Printf("cache error (marshal key=%s): %v", key, err)
		return
	}

	if err := s.redis.Set(ctx, key, payload, s.cacheTTL).Err(); err != nil {
		log.Printf("cache error (set key=%s): %v", key, err)
		return
	}

	log.Printf("cache set key=%s ttl=%s", key, s.cacheTTL)
}

func (s *FlightServer) getCachedFlight(ctx context.Context, key string) (*model.Flight, bool) {
	if s.redis == nil {
		return nil, false
	}

	payload, err := s.redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		log.Printf("cache miss key=%s", key)
		return nil, false
	}
	if err != nil {
		log.Printf("cache error (get key=%s): %v", key, err)
		return nil, false
	}

	var flight model.Flight
	if err := json.Unmarshal([]byte(payload), &flight); err != nil {
		log.Printf("cache error (unmarshal key=%s): %v", key, err)
		return nil, false
	}

	log.Printf("cache hit key=%s", key)
	return &flight, true
}

func (s *FlightServer) setCachedFlight(ctx context.Context, key string, flight *model.Flight) {
	if s.redis == nil {
		return
	}

	payload, err := json.Marshal(flight)
	if err != nil {
		log.Printf("cache error (marshal key=%s): %v", key, err)
		return
	}

	if err := s.redis.Set(ctx, key, payload, s.cacheTTL).Err(); err != nil {
		log.Printf("cache error (set key=%s): %v", key, err)
		return
	}

	log.Printf("cache set key=%s ttl=%s", key, s.cacheTTL)
}

func (s *FlightServer) invalidateCaches(ctx context.Context, flightID string) {
	if s.redis == nil {
		return
	}

	flightKey := flightCacheKey(flightID)
	if err := s.redis.Del(ctx, flightKey).Err(); err != nil {
		log.Printf("cache error (del key=%s): %v", flightKey, err)
	} else {
		log.Printf("cache invalidate key=%s", flightKey)
	}

	s.invalidateSearchCaches(ctx)
}

func (s *FlightServer) invalidateSearchCaches(ctx context.Context) {
	if s.redis == nil {
		return
	}

	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, "search:*", 100).Result()
		if err != nil {
			log.Printf("cache error (scan search:*): %v", err)
			return
		}

		if len(keys) > 0 {
			if err := s.redis.Del(ctx, keys...).Err(); err != nil {
				log.Printf("cache error (del search keys): %v", err)
			} else {
				log.Printf("cache invalidate pattern=search:* deleted=%d", len(keys))
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

func toProtoFlight(f *model.Flight) *flightpb.FlightInfo {
	return &flightpb.FlightInfo{
		Id:                 f.ID,
		FlightNumber:       f.FlightNumber,
		Airline:            f.Airline,
		OriginAirport:      f.OriginAirport,
		DestinationAirport: f.DestinationAirport,
		DepartureTime:      timestamppb.New(f.DepartureTime),
		ArrivalTime:        timestamppb.New(f.ArrivalTime),
		TotalSeats:         f.TotalSeats,
		AvailableSeats:     f.AvailableSeats,
		Price:              f.Price,
		Status:             toProtoFlightStatus(f.Status),
	}
}

func toProtoFlightStatus(s string) flightpb.FlightStatus {
	switch s {
	case "SCHEDULED":
		return flightpb.FlightStatus_FLIGHT_STATUS_SCHEDULED
	case "DEPARTED":
		return flightpb.FlightStatus_FLIGHT_STATUS_DEPARTED
	case "CANCELLED":
		return flightpb.FlightStatus_FLIGHT_STATUS_CANCELLED
	case "COMPLETED":
		return flightpb.FlightStatus_FLIGHT_STATUS_COMPLETED
	default:
		return flightpb.FlightStatus_FLIGHT_STATUS_UNSPECIFIED
	}
}

func toProtoReservation(r *model.SeatReservation) *flightpb.Reservation {
	return &flightpb.Reservation{
		Id:        r.ID,
		FlightId:  r.FlightID,
		BookingId: r.BookingID,
		SeatCount: r.SeatCount,
		Status:    toProtoReservationStatus(r.Status),
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
}

func toProtoReservationStatus(s string) flightpb.ReservationStatus {
	switch s {
	case "ACTIVE":
		return flightpb.ReservationStatus_RESERVATION_STATUS_ACTIVE
	case "RELEASED":
		return flightpb.ReservationStatus_RESERVATION_STATUS_RELEASED
	case "EXPIRED":
		return flightpb.ReservationStatus_RESERVATION_STATUS_EXPIRED
	default:
		return flightpb.ReservationStatus_RESERVATION_STATUS_UNSPECIFIED
	}
}

func mapRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrFlightNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, repository.ErrFlightNotScheduled):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, repository.ErrNotEnoughSeats):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, repository.ErrReservationExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, repository.ErrReservationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, repository.ErrReservationNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		log.Printf("unexpected error: %v", err)
		return status.Error(codes.Internal, "internal error")
	}
}
