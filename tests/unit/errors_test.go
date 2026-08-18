package unit_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

func TestWriteErrorDoesNotExposeInternalError(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(request.Context())
	handler := middleware.Correlation(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		httptransport.WriteError(writer, request, errors.New("database password leaked"))
	}))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if body := recorder.Body.String(); strings.Contains(body, "database password") || !strings.Contains(body, "request_id") {
		t.Fatalf("unsafe error body: %s", body)
	}
}
