package httperr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gnemade360/go-server/httperr"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func write(t *testing.T, err error) (*httptest.ResponseRecorder, errorBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	httperr.Write(rec, err)
	var got errorBody
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &got); decodeErr != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), decodeErr)
	}
	return rec, got
}

func TestConstructorsSetStatusAndCode(t *testing.T) {
	tests := []struct {
		name       string
		err        *httperr.Error
		wantStatus int
		wantCode   string
	}{
		{"BadRequest", httperr.BadRequest("m"), http.StatusBadRequest, "bad_request"},
		{"Invalid", httperr.Invalid("m"), http.StatusBadRequest, "invalid"},
		{"Unauthorized", httperr.Unauthorized("m"), http.StatusUnauthorized, "unauthorized"},
		{"Forbidden", httperr.Forbidden("m"), http.StatusForbidden, "forbidden"},
		{"NotFound", httperr.NotFound("m"), http.StatusNotFound, "not_found"},
		{"Conflict", httperr.Conflict("m"), http.StatusConflict, "conflict"},
		{"Internal", httperr.Internal(errors.New("boom")), http.StatusInternalServerError, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Status != tt.wantStatus || tt.err.Code != tt.wantCode {
				t.Errorf("%s = (%d, %q), want (%d, %q)", tt.name, tt.err.Status, tt.err.Code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestWriteAnswersWithStatusAndBody(t *testing.T) {
	rec, got := write(t, httperr.NotFound("order 42 not found"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", nosniff)
	}
	if got.Error.Code != "not_found" || got.Error.Message != "order 42 not found" {
		t.Errorf("body = %+v, want code not_found and the message", got.Error)
	}
}

func TestWriteFindsErrorInsideWrappedChain(t *testing.T) {
	wrapped := fmt.Errorf("load order: %w", httperr.Conflict("order already placed"))

	rec, got := write(t, wrapped)

	if rec.Code != http.StatusConflict || got.Error.Code != "conflict" {
		t.Errorf("Write(wrapped conflict) = (%d, %q), want (409, conflict)", rec.Code, got.Error.Code)
	}
}

func TestWriteHidesUnexpectedErrorDetails(t *testing.T) {
	rec, got := write(t, errors.New("pq: password authentication failed for user app"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if got.Error.Code != "internal" || got.Error.Message != "internal error" {
		t.Errorf("body = %+v, want internal / internal error", got.Error)
	}
}

func TestWriteNeverSendsMessageOfA5xx(t *testing.T) {
	_, got := write(t, httperr.New(http.StatusServiceUnavailable, "unavailable", "db host 10.0.0.5 down"))

	if got.Error.Message != "internal error" {
		t.Errorf("message = %q, want internal error", got.Error.Message)
	}
}

func TestWriteNilIsInternal(t *testing.T) {
	rec, _ := write(t, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Write(nil) status = %d, want 500", rec.Code)
	}
}

func TestCauseIsKeptForErrorsIs(t *testing.T) {
	errNoStock := errors.New("ordering: no stock")
	err := httperr.Conflict("not enough stock").WithCause(errNoStock)

	if !errors.Is(err, errNoStock) {
		t.Error("errors.Is(err, errNoStock) = false, want true")
	}
	if got := err.Error(); got != "httperr: conflict: not enough stock: ordering: no stock" {
		t.Errorf("Error() = %q", got)
	}
}

func TestWithCauseDoesNotChangeTheOriginal(t *testing.T) {
	base := httperr.NotFound("missing")
	_ = base.WithCause(errors.New("cause"))

	if base.Cause != nil {
		t.Error("WithCause changed the receiver")
	}
}

func TestFrom(t *testing.T) {
	if httperr.From(nil) != nil {
		t.Error("From(nil) != nil")
	}
	notFound := httperr.NotFound("x")
	if got := httperr.From(fmt.Errorf("w: %w", notFound)); got != notFound {
		t.Errorf("From(wrapped) = %v, want the same *Error", got)
	}
	plain := errors.New("plain")
	got := httperr.From(plain)
	if got.Status != http.StatusInternalServerError || !errors.Is(got, plain) {
		t.Errorf("From(plain) = %v, want Internal keeping the cause", got)
	}
}
