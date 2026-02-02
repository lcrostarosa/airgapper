package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
)

// handleGetSchedule returns the current backup schedule
func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	info := s.statusSvc.GetScheduleInfo()
	jsonResponse(w, http.StatusOK, info)
}

// handleUpdateSchedule updates the backup schedule.
// If the scheduler is running, it performs a hot-reload without restart.
func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var body ScheduleUpdateBody
	if err := decodeJSON(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate the schedule expression if provided
	var newSchedule *scheduler.Schedule
	if body.Schedule != "" {
		var err error
		// Try enhanced parsing first (supports ranges/steps/lists)
		newSchedule, err = scheduler.ParseScheduleEnhanced(body.Schedule)
		if err != nil {
			// Fall back to basic parsing for error message consistency
			_, err = scheduler.ParseSchedule(body.Schedule)
			if err != nil {
				jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid schedule: %v", err))
				return
			}
		}
	}

	// Update config
	if err := s.statusSvc.UpdateSchedule(body.Schedule, body.Paths); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Attempt hot-reload if scheduler is running and we have a new schedule
	hotReloaded := false
	if newSchedule != nil && s.statusSvc.HasScheduler() {
		s.statusSvc.HotReloadSchedule(newSchedule)
		hotReloaded = true
	}

	response := ScheduleUpdatedDTO{
		Status: "updated",
	}

	if hotReloaded {
		response.Message = "Schedule updated and hot-reloaded"
		hotReloadedVal := true
		response.HotReloaded = &hotReloadedVal
	} else if newSchedule != nil {
		response.Message = "Schedule updated. Restart server to apply."
		hotReloadedVal := false
		response.HotReloaded = &hotReloadedVal
	} else {
		response.Message = "Paths updated"
	}

	jsonResponse(w, http.StatusOK, response)
}

// handleGetBackupHistory returns recent backup results
func (s *Server) handleGetBackupHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history := s.statusSvc.GetBackupHistory(limit)
	jsonResponse(w, http.StatusOK, ToBackupHistoryDTO(history))
}
