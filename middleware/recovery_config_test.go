package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryWithConfig_CustomRespondBeforeWrite(t *testing.T) {
	handler := RecoveryWithConfig(RecoveryConfig{
		Log: func(*http.Request, any, []byte) {},
		Respond: func(w http.ResponseWriter, r *http.Request, recovered any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal"}`))
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"error":"internal"}` {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
}

func TestRecoveryWithConfig_AfterWriteLogsAndAbortsWithoutRespond(t *testing.T) {
	respondCalled := false
	var loggedValue any
	handler := RecoveryWithConfig(RecoveryConfig{
		Log: func(_ *http.Request, recovered any, _ []byte) {
			loggedValue = recovered
		},
		Respond: func(http.ResponseWriter, *http.Request, any) {
			respondCalled = true
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: x\n\n"))
		panic("mid-stream")
	}))

	rec, recovered := serveRecorderAndRecover(handler)

	if recovered != http.ErrAbortHandler {
		t.Errorf("expected http.ErrAbortHandler, got %v", recovered)
	}
	if loggedValue != "mid-stream" {
		t.Errorf("expected Log to get the original value, got %v", loggedValue)
	}
	if respondCalled {
		t.Error("expected Respond not to be called once the response has started")
	}
	if rec.Body.String() != "data: x\n\n" {
		t.Errorf("expected body %q only, got %q", "data: x\n\n", rec.Body.String())
	}
}

func TestRecoveryWithConfig_LogGetsRequestValueAndStack(t *testing.T) {
	var gotPath string
	var gotValue any
	var gotStack []byte
	handler := RecoveryWithConfig(RecoveryConfig{
		Log: func(r *http.Request, recovered any, stack []byte) {
			gotPath = r.URL.Path
			gotValue = recovered
			gotStack = stack
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders", nil))

	if gotPath != "/orders" {
		t.Errorf("expected the request, got path %q", gotPath)
	}
	if gotValue != "boom" {
		t.Errorf("expected the recovered value, got %v", gotValue)
	}
	if !strings.Contains(string(gotStack), "goroutine") {
		t.Errorf("expected a stack trace, got %q", gotStack)
	}
}

func TestRecoveryWithConfig_ReraisesErrAbortHandlerWithoutLogging(t *testing.T) {
	logCalled := false
	handler := RecoveryWithConfig(RecoveryConfig{
		Log: func(*http.Request, any, []byte) { logCalled = true },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	recovered := serveAndRecover(handler)

	if recovered != http.ErrAbortHandler {
		t.Errorf("expected http.ErrAbortHandler to be re-panicked, got %v", recovered)
	}
	if logCalled {
		t.Error("expected Log not to be called for ErrAbortHandler")
	}
}
