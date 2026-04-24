package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/online-cinema/producer/internal/kafka"
	"github.com/online-cinema/producer/internal/model"
)

type Handler struct {
	producer *kafka.Producer
	logger   *zap.Logger
}

func NewHandler(producer *kafka.Producer, logger *zap.Logger) *Handler {
	return &Handler{
		producer: producer,
		logger:   logger,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		v1.POST("/events", h.PublishEvent)
		v1.POST("/events/batch", h.PublishBatch)
	}
	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)
}

func (h *Handler) PublishEvent(c *gin.Context) {
	var req model.EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "validation failed",
			Details: err.Error(),
		})
		return
	}

	pbEvent := req.ToProto()

	if err := h.producer.Publish(c.Request.Context(), pbEvent); err != nil {
		h.logger.Error("publish failed",
			zap.String("event_id", req.EventID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Error: "failed to publish event",
		})
		return
	}

	c.JSON(http.StatusOK, model.EventResponse{
		EventID: req.EventID,
		Status:  "published",
	})
}

func (h *Handler) PublishBatch(c *gin.Context) {
	var req model.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid request body",
			Details: err.Error(),
		})
		return
	}

	resp := model.BatchResponse{
		Total:   len(req.Events),
		Results: make([]model.EventResponse, 0, len(req.Events)),
	}

	for _, event := range req.Events {
		if err := event.Validate(); err != nil {
			resp.Failed++
			resp.Results = append(resp.Results, model.EventResponse{
				EventID: event.EventID,
				Status:  "validation_error: " + err.Error(),
			})
			continue
		}

		pbEvent := event.ToProto()
		if err := h.producer.Publish(c.Request.Context(), pbEvent); err != nil {
			resp.Failed++
			resp.Results = append(resp.Results, model.EventResponse{
				EventID: event.EventID,
				Status:  "publish_error",
			})
			continue
		}

		resp.Published++
		resp.Results = append(resp.Results, model.EventResponse{
			EventID: event.EventID,
			Status:  "published",
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Ready(c *gin.Context) {
	if err := h.producer.Health(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
