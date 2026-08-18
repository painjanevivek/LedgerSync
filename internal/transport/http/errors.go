package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

// PublicError is the allowlist of errors that may cross the HTTP boundary.
// Internal errors are logged with a correlation ID and become a generic error.
type PublicError struct {
	Status  int
	Code    string
	Message string
}

func (e *PublicError) Error() string { return e.Code }

var (
	ErrBadRequest   = &PublicError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "The request is invalid."}
	ErrUnauthorized = &PublicError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Authentication is required."}
	ErrForbidden    = &PublicError{Status: http.StatusForbidden, Code: "forbidden", Message: "You cannot perform this action."}
	ErrNotFound     = &PublicError{Status: http.StatusNotFound, Code: "not_found", Message: "The requested resource was not found."}
	ErrConflict     = &PublicError{Status: http.StatusConflict, Code: "conflict", Message: "The request conflicts with its prior state."}
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func WriteError(writer http.ResponseWriter, request *http.Request, err error) {
	public := &PublicError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "The service could not complete the request."}
	var candidate *PublicError
	if errors.As(err, &candidate) {
		public = candidate
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(public.Status)
	_ = json.NewEncoder(writer).Encode(errorEnvelope{Error: errorBody{
		Code: public.Code, Message: public.Message, RequestID: middleware.CorrelationID(request.Context()),
	}})
}
