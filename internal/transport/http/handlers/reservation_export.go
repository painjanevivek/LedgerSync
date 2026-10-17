package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

func (h *ReservationHandler) Export(w http.ResponseWriter, r *http.Request) {
	p, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	if identity.RequireScope(p, "exports:read") != nil {
		writeScopeDenial(w, r, h.audit, p, "exports:read")
		return
	}
	if r.URL.RawQuery != "" {
		writeReservationError(w, r, hold.ErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	raw, err := h.service.Export(ctx, reservationScope(p))
	if errors.Is(err, hold.ErrLimit) {
		httptransport.WriteError(w, r, &httptransport.PublicError{Status: 413, Code: "reservation_export_limit", Message: "The authorized snapshot exceeds 2000 holds or 2 MiB. No CSV was returned."})
		return
	}
	if err != nil {
		writeReservationError(w, r, err)
		return
	}
	if h.audit == nil {
		writeReservationError(w, r, hold.ErrUnavailable)
		return
	}
	reference := middleware.CorrelationID(r.Context())
	err = h.audit.Record(ctx, db.AuditEvent{TenantID: p.TenantID, ActorSubjectID: p.SubjectID, EventType: "reservation.exported", TargetType: "reservation", Outcome: "succeeded", CorrelationID: reference, Metadata: map[string]string{"operation": "snapshot_export", "max_records": strconv.Itoa(hold.MaxExportHolds)}})
	if err != nil {
		writeReservationError(w, r, hold.ErrUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="reservation-obligations.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Audit-Reference", reference)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}
