package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/cypherlabdev/odds-optimizer-service/internal/models"
	"github.com/cypherlabdev/odds-optimizer-service/internal/service"
)

// OddsHandler handles HTTP requests for optimized odds
type OddsHandler struct {
	service *service.OptimizerService
	logger  zerolog.Logger
}

// NewOddsHandler creates a new odds HTTP handler
func NewOddsHandler(service *service.OptimizerService, logger zerolog.Logger) *OddsHandler {
	return &OddsHandler{
		service: service,
		logger:  logger.With().Str("component", "odds_handler").Logger(),
	}
}

// RegisterRoutes registers HTTP routes with the provided mux
func (h *OddsHandler) RegisterRoutes(mux *http.ServeMux) {
	// GET /api/v1/odds/:event_id/:market/:selection - Get specific optimized odds
	mux.HandleFunc("/api/v1/odds/", h.handleGetOdds)

	// GET /api/v1/events/:event_id/odds - Get all odds for an event
	mux.HandleFunc("/api/v1/events/", h.handleGetEventOdds)
}

// handleGetOdds handles GET /api/v1/odds/:event_id/:market/:selection
func (h *OddsHandler) handleGetOdds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /api/v1/odds/:event_id/:market/:selection
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/odds/")
	parts := strings.Split(path, "/")

	if len(parts) != 3 {
		h.errorResponse(w, http.StatusBadRequest, "invalid path: expected /api/v1/odds/:event_id/:market/:selection")
		return
	}

	eventID := parts[0]
	market := parts[1]
	selection := parts[2]

	if eventID == "" || market == "" || selection == "" {
		h.errorResponse(w, http.StatusBadRequest, "event_id, market, and selection are required")
		return
	}

	// Get optimized odds from service
	odds, err := h.service.GetOptimizedOdds(r.Context(), eventID, market, selection)
	if err != nil {
		h.logger.Debug().
			Err(err).
			Str("event_id", eventID).
			Str("market", market).
			Str("selection", selection).
			Msg("odds not found")
		h.errorResponse(w, http.StatusNotFound, "odds not found")
		return
	}

	h.jsonResponse(w, http.StatusOK, odds)
}

// handleGetEventOdds handles GET /api/v1/events/:event_id/odds
func (h *OddsHandler) handleGetEventOdds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse path: /api/v1/events/:event_id/odds
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	parts := strings.Split(path, "/")

	if len(parts) != 2 || parts[1] != "odds" {
		h.errorResponse(w, http.StatusBadRequest, "invalid path: expected /api/v1/events/:event_id/odds")
		return
	}

	eventID := parts[0]
	if eventID == "" {
		h.errorResponse(w, http.StatusBadRequest, "event_id is required")
		return
	}

	// Get all odds for event from service
	oddsList, err := h.service.GetOptimizedOddsByEvent(r.Context(), eventID)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("event_id", eventID).
			Msg("failed to retrieve event odds")
		h.errorResponse(w, http.StatusInternalServerError, "failed to retrieve odds")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"event_id": eventID,
		"count":    len(oddsList),
		"odds":     oddsList,
	})
}

// jsonResponse writes a JSON response
func (h *OddsHandler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode JSON response")
	}
}

// errorResponse writes a JSON error response
func (h *OddsHandler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{
		"error": message,
	})
}

// OddsResponse represents the API response for odds
type OddsResponse struct {
	EventID       string  `json:"event_id"`
	EventName     string  `json:"event_name"`
	Sport         string  `json:"sport"`
	Competition   string  `json:"competition"`
	Market        string  `json:"market"`
	Selection     string  `json:"selection"`
	OptimizedBack string  `json:"optimized_back"`
	OptimizedLay  string  `json:"optimized_lay"`
	OriginalBack  string  `json:"original_back"`
	OriginalLay   string  `json:"original_lay"`
	Margin        string  `json:"margin"`
	Confidence    float64 `json:"confidence"`
	OptimizedAt   string  `json:"optimized_at"`
}

// ToOddsResponse converts OptimizedOdds to API response format
func ToOddsResponse(odds *models.OptimizedOdds) *OddsResponse {
	return &OddsResponse{
		EventID:       odds.EventID,
		EventName:     odds.EventName,
		Sport:         odds.Sport,
		Competition:   odds.Competition,
		Market:        odds.Market,
		Selection:     odds.Selection,
		OptimizedBack: odds.OptimizedBack.String(),
		OptimizedLay:  odds.OptimizedLay.String(),
		OriginalBack:  odds.OriginalBack.String(),
		OriginalLay:   odds.OriginalLay.String(),
		Margin:        odds.Margin.String(),
		Confidence:    odds.Confidence,
		OptimizedAt:   odds.OptimizedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
