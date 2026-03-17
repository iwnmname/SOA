package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"hw3/booking-service/internal/model"
	"hw3/booking-service/internal/repository"
	flightpb "hw3/gen/flight"
)

type Handler struct {
	repo         *repository.BookingRepository
	flightClient flightpb.FlightServiceClient
}

func NewHandler(repo *repository.BookingRepository, fc flightpb.FlightServiceClient) *Handler {
	return &Handler{repo: repo, flightClient: fc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/flights", h.SearchFlights)
	r.Get("/flights/{id}", h.GetFlight)

	r.Post("/bookings", h.CreateBooking)
	r.Get("/bookings/{id}", h.GetBooking)
	r.Post("/bookings/{id}/cancel", h.CancelBooking)
	r.Get("/bookings", h.ListBookings)

	return r
}

type FlightResponse struct {
	ID                 string  `json:"id"`
	FlightNumber       string  `json:"flight_number"`
	Airline            string  `json:"airline"`
	OriginAirport      string  `json:"origin_airport"`
	DestinationAirport string  `json:"destination_airport"`
	DepartureTime      string  `json:"departure_time"`
	ArrivalTime        string  `json:"arrival_time"`
	TotalSeats         int32   `json:"total_seats"`
	AvailableSeats     int32   `json:"available_seats"`
	Price              float64 `json:"price"`
	Status             string  `json:"status"`
}

func toFlightResponse(f *flightpb.FlightInfo) *FlightResponse {
	return &FlightResponse{
		ID:                 f.Id,
		FlightNumber:       f.FlightNumber,
		Airline:            f.Airline,
		OriginAirport:      f.OriginAirport,
		DestinationAirport: f.DestinationAirport,
		DepartureTime:      f.DepartureTime.AsTime().Format(time.RFC3339),
		ArrivalTime:        f.ArrivalTime.AsTime().Format(time.RFC3339),
		TotalSeats:         f.TotalSeats,
		AvailableSeats:     f.AvailableSeats,
		Price:              f.Price,
		Status:             f.Status.String(),
	}
}

func (h *Handler) SearchFlights(w http.ResponseWriter, r *http.Request) {
	origin := r.URL.Query().Get("origin")
	destination := r.URL.Query().Get("destination")
	dateStr := r.URL.Query().Get("date")

	if origin == "" || destination == "" {
		writeError(w, http.StatusBadRequest, "origin and destination are required")
		return
	}

	req := &flightpb.SearchFlightsRequest{
		Origin:      origin,
		Destination: destination,
	}

	if dateStr != "" {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
			return
		}
		req.Date = timestamppb.New(t)
	}

	resp, err := h.flightClient.SearchFlights(r.Context(), req)
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	flights := make([]*FlightResponse, 0, len(resp.Flights))
	for _, f := range resp.Flights {
		flights = append(flights, toFlightResponse(f))
	}

	writeJSON(w, http.StatusOK, flights)
}

func (h *Handler) GetFlight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resp, err := h.flightClient.GetFlight(r.Context(), &flightpb.GetFlightRequest{FlightId: id})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toFlightResponse(resp.Flight))
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req model.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" || req.FlightID == "" || req.PassengerName == "" || req.PassengerEmail == "" || req.SeatCount <= 0 {
		writeError(w, http.StatusBadRequest, "all fields are required and seat_count must be positive")
		return
	}

	flightResp, err := h.flightClient.GetFlight(r.Context(), &flightpb.GetFlightRequest{
		FlightId: req.FlightID,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	bookingID := uuid.New().String()

	_, err = h.flightClient.ReserveSeats(r.Context(), &flightpb.ReserveSeatsRequest{
		FlightId:  req.FlightID,
		BookingId: bookingID,
		SeatCount: req.SeatCount,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	booking := &model.Booking{
		ID:             bookingID,
		UserID:         req.UserID,
		FlightID:       req.FlightID,
		PassengerName:  req.PassengerName,
		PassengerEmail: req.PassengerEmail,
		SeatCount:      req.SeatCount,
		TotalPrice:     float64(req.SeatCount) * flightResp.Flight.Price,
		Status:         "CONFIRMED",
	}

	if err := h.repo.Create(r.Context(), booking); err != nil {
		log.Printf("failed to save booking %s: %v", bookingID, err)

		releaseCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if releaseErr := h.releaseReservationWithRetry(releaseCtx, bookingID, 3); releaseErr != nil {
			log.Printf("failed to compensate reservation for booking %s: %v", bookingID, releaseErr)
		}

		writeError(w, http.StatusInternalServerError, "failed to create booking")
		return
	}

	writeJSON(w, http.StatusCreated, booking)
}

func (h *Handler) GetBooking(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	booking, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if booking == nil {
		writeError(w, http.StatusNotFound, "booking not found")
		return
	}

	writeJSON(w, http.StatusOK, booking)
}

func (h *Handler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	booking, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if booking == nil {
		writeError(w, http.StatusNotFound, "booking not found")
		return
	}
	if booking.Status != "CONFIRMED" {
		writeError(w, http.StatusConflict, "booking is not in CONFIRMED status")
		return
	}

	_, err = h.flightClient.ReleaseReservation(r.Context(), &flightpb.ReleaseReservationRequest{
		BookingId: id,
	})
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	if err := h.repo.UpdateStatus(r.Context(), id, "CANCELLED"); err != nil {
		log.Printf("failed to update booking status %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to cancel booking")
		return
	}

	booking.Status = "CANCELLED"
	writeJSON(w, http.StatusOK, booking)
}

func (h *Handler) ListBookings(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	bookings, err := h.repo.GetByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if bookings == nil {
		bookings = []*model.Booking{}
	}

	writeJSON(w, http.StatusOK, bookings)
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func handleGRPCError(w http.ResponseWriter, err error) {
	st, _ := status.FromError(err)
	writeError(w, grpcToHTTP(st.Code()), st.Message())
}

func grpcToHTTP(code codes.Code) int {
	switch code {
	case codes.NotFound:
		return http.StatusNotFound
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.ResourceExhausted:
		return http.StatusConflict
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func (h *Handler) releaseReservationWithRetry(ctx context.Context, bookingID string, attempts int) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		_, err := h.flightClient.ReleaseReservation(ctx, &flightpb.ReleaseReservationRequest{BookingId: bookingID})
		if err == nil {
			return nil
		}

		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.NotFound || st.Code() == codes.FailedPrecondition) {
			return nil
		}

		lastErr = err

		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i+1) * 250 * time.Millisecond):
			}
		}
	}

	if lastErr == nil {
		return context.DeadlineExceeded
	}

	return lastErr
}

