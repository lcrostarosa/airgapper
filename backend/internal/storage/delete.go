package storage

import (
	"os"
	"strings"
)

// checkDeleteAllowed checks if deletion is allowed based on policy and tickets
func (s *Server) checkDeleteAllowed(filePath string) (bool, string) {
	return s.checkDeleteAllowedWithTicket(filePath, "", "")
}

// checkDeleteAllowedWithTicket checks if deletion is allowed, optionally with a ticket
func (s *Server) checkDeleteAllowedWithTicket(filePath, snapshotID, ticketID string) (bool, string) {
	// Always check append-only first
	if s.appendOnly {
		return false, "delete not allowed in append-only mode"
	}

	// Check if ticket system requires tickets for this deletion
	if s.ticketManager != nil && s.verificationConfig != nil && s.verificationConfig.IsTicketsEnabled() {
		// Determine if this is a snapshot deletion that requires a ticket
		isSnapshot := strings.Contains(filePath, "/snapshots/")
		requireTicket := (isSnapshot && s.verificationConfig.Tickets.RequireForSnapshots)

		if requireTicket {
			// If a ticket ID was provided, validate it
			if ticketID != "" {
				ticket := s.ticketManager.GetTicket(ticketID)
				if ticket == nil {
					return false, "invalid ticket ID"
				}
				// Ticket exists - allow deletion (ticket was already validated on registration)
			} else {
				// No ticket provided - try to find a valid one
				foundTicketID, err := s.ticketManager.ValidateDelete(filePath, snapshotID)
				if err != nil {
					return false, "deletion requires valid ticket: " + err.Error()
				}
				ticketID = foundTicketID
			}

			// Record ticket usage
			if ticketID != "" {
				_, _ = s.ticketManager.RecordUsage(ticketID, []string{filePath})
			}
		}
	}

	if s.policy == nil {
		return true, ""
	}

	// Get file creation time
	info, err := os.Stat(filePath)
	if err != nil {
		return false, "file not found"
	}

	// Use modification time as proxy for creation time
	// (Note: Go doesn't have a portable way to get creation time)
	fileTime := info.ModTime()

	return s.policy.CanDelete(fileTime)
}
