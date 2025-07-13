package filter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mock Filter implementation for testing
type mockFilter struct {
	order     int
	executed  bool
	headerKey string
	value     string
}

func (m *mockFilter) Order() int {
	return m.order
}

func (m *mockFilter) Do(w http.ResponseWriter, r *http.Request, next http.Handler) {
	m.executed = true
	w.Header().Set(m.headerKey, m.value)
	next.ServeHTTP(w, r)
}

// Mock ConditionalFilter implementation for testing
type mockConditionalFilter struct {
	mockFilter
	matchCondition func(*http.Request) bool
}

func (m *mockConditionalFilter) Match(r *http.Request) bool {
	return m.matchCondition(r)
}

func TestFilterFn_Type(t *testing.T) {
	// Test that FilterFn has the correct signature
	var fn FilterFn = func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		w.Header().Set("X-Test", "value")
		next.ServeHTTP(w, r)
	}

	if fn == nil {
		t.Error("FilterFn should not be nil")
	}
}

func TestFilter_Interface(t *testing.T) {
	// Test that mockFilter implements Filter interface
	var filter Filter = &mockFilter{
		order:     1,
		headerKey: "X-Test",
		value:     "test-value",
	}

	if filter.Order() != 1 {
		t.Errorf("Expected order 1, got %d", filter.Order())
	}

	// Test Do method
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	filter.Do(w, req, nextHandler)

	mockF := filter.(*mockFilter)
	if !mockF.executed {
		t.Error("Filter Do method was not executed")
	}

	if w.Header().Get("X-Test") != "test-value" {
		t.Errorf("Expected header X-Test: test-value, got %s", w.Header().Get("X-Test"))
	}
}

func TestConditionalFilter_Interface(t *testing.T) {
	// Test that mockConditionalFilter implements ConditionalFilter interface
	var filter ConditionalFilter = &mockConditionalFilter{
		mockFilter: mockFilter{
			order:     2,
			headerKey: "X-Conditional",
			value:     "conditional-value",
		},
		matchCondition: func(r *http.Request) bool {
			return r.URL.Path == "/match"
		},
	}

	if filter.Order() != 2 {
		t.Errorf("Expected order 2, got %d", filter.Order())
	}

	// Test Match method - should match
	req1 := httptest.NewRequest("GET", "/match", nil)
	if !filter.Match(req1) {
		t.Error("Expected filter to match request to /match")
	}

	// Test Match method - should not match
	req2 := httptest.NewRequest("GET", "/nomatch", nil)
	if filter.Match(req2) {
		t.Error("Expected filter to not match request to /nomatch")
	}
}

func TestFnFilter(t *testing.T) {
	executed := false
	fn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executed = true
		w.Header().Set("X-FnFilter", "executed")
		next.ServeHTTP(w, r)
	}

	fnFilter := fnFilter{order: 5, fn: fn}

	if fnFilter.Order() != 5 {
		t.Errorf("Expected order 5, got %d", fnFilter.Order())
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	fnFilter.Do(w, req, nextHandler)

	if !executed {
		t.Error("FilterFn was not executed")
	}

	if w.Header().Get("X-FnFilter") != "executed" {
		t.Error("Expected X-FnFilter header to be set")
	}
}

func TestNewFilterFn(t *testing.T) {
	executed := false
	fn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executed = true
		w.Header().Set("X-NewFilter", "created")
		next.ServeHTTP(w, r)
	}

	filter := NewFilterFn(10, fn)

	if filter.Order() != 10 {
		t.Errorf("Expected order 10, got %d", filter.Order())
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	filter.Do(w, req, nextHandler)

	if !executed {
		t.Error("FilterFn was not executed")
	}

	if w.Header().Get("X-NewFilter") != "created" {
		t.Error("Expected X-NewFilter header to be set")
	}
}

func TestAdapter_RegularFilter(t *testing.T) {
	filter := &mockFilter{
		order:     1,
		headerKey: "X-Adapter-Test",
		value:     "adapter-value",
	}

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next", "called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !filter.executed {
		t.Error("Filter was not executed")
	}

	if w.Header().Get("X-Adapter-Test") != "adapter-value" {
		t.Error("Expected filter header to be set")
	}

	if w.Header().Get("X-Next") != "called" {
		t.Error("Expected next handler to be called")
	}
}

func TestAdapter_ConditionalFilter_Matching(t *testing.T) {
	filter := &mockConditionalFilter{
		mockFilter: mockFilter{
			order:     1,
			headerKey: "X-Conditional-Test",
			value:     "conditional-value",
		},
		matchCondition: func(r *http.Request) bool {
			return r.URL.Path == "/conditional"
		},
	}

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next", "called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/conditional", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !filter.executed {
		t.Error("Conditional filter should have been executed for matching request")
	}

	if w.Header().Get("X-Conditional-Test") != "conditional-value" {
		t.Error("Expected conditional filter header to be set")
	}

	if w.Header().Get("X-Next") != "called" {
		t.Error("Expected next handler to be called")
	}
}

func TestAdapter_ConditionalFilter_NotMatching(t *testing.T) {
	filter := &mockConditionalFilter{
		mockFilter: mockFilter{
			order:     1,
			headerKey: "X-Conditional-Test",
			value:     "conditional-value",
		},
		matchCondition: func(r *http.Request) bool {
			return r.URL.Path == "/conditional"
		},
	}

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next", "called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/other", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if filter.executed {
		t.Error("Conditional filter should not have been executed for non-matching request")
	}

	if w.Header().Get("X-Conditional-Test") != "" {
		t.Error("Expected no conditional filter header to be set")
	}

	if w.Header().Get("X-Next") != "called" {
		t.Error("Expected next handler to be called")
	}
}

func TestAdapter_ChainedFilters(t *testing.T) {
	filter1 := &mockFilter{
		order:     1,
		headerKey: "X-Filter-1",
		value:     "first",
	}

	filter2 := &mockFilter{
		order:     2,
		headerKey: "X-Filter-2",
		value:     "second",
	}

	middleware1 := Adapter(filter1)
	middleware2 := Adapter(filter2)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Final", "done")
		w.WriteHeader(http.StatusOK)
	})

	// Chain middlewares: middleware1(middleware2(nextHandler))
	handler := middleware1(middleware2(nextHandler))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !filter1.executed {
		t.Error("Filter 1 was not executed")
	}

	if !filter2.executed {
		t.Error("Filter 2 was not executed")
	}

	if w.Header().Get("X-Filter-1") != "first" {
		t.Error("Expected X-Filter-1 header to be set")
	}

	if w.Header().Get("X-Filter-2") != "second" {
		t.Error("Expected X-Filter-2 header to be set")
	}

	if w.Header().Get("X-Final") != "done" {
		t.Error("Expected final handler to be called")
	}
}

func TestAdapter_FilterModifiesRequest(t *testing.T) {
	filter := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		// Modify the request by adding a header
		r.Header.Set("X-Modified", "true")
		next.ServeHTTP(w, r)
	})

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modified := r.Header.Get("X-Modified")
		w.Header().Set("X-Received-Modified", modified)
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Received-Modified") != "true" {
		t.Error("Expected filter to modify request")
	}
}

func TestAdapter_FilterModifiesResponse(t *testing.T) {
	filter := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		// Add header before calling next
		w.Header().Set("X-Pre-Process", "before")
		next.ServeHTTP(w, r)
		// Note: Cannot add headers after next.ServeHTTP if it writes to the response
	})

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", "executed")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Pre-Process") != "before" {
		t.Error("Expected filter to add pre-process header")
	}

	if w.Header().Get("X-Handler") != "executed" {
		t.Error("Expected handler to add its header")
	}
}

func BenchmarkAdapter(b *testing.B) {
	filter := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		w.Header().Set("X-Benchmark", "test")
		next.ServeHTTP(w, r)
	})

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkNewFilterFn(b *testing.B) {
	fn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		next.ServeHTTP(w, r)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewFilterFn(i, fn)
	}
}

func TestAdapter_ErrorHandling(t *testing.T) {
	filter := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		// Simulate some processing that could fail
		if r.URL.Path == "/error" {
			http.Error(w, "Filter error", http.StatusBadRequest)
			return // Don't call next
		}
		next.ServeHTTP(w, r)
	})

	middleware := Adapter(filter)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next-Called", "true")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	// Test error case
	req1 := httptest.NewRequest("GET", "/error", nil)
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w1.Code)
	}

	if w1.Header().Get("X-Next-Called") != "" {
		t.Error("Expected next handler not to be called on error")
	}

	// Test success case
	req2 := httptest.NewRequest("GET", "/success", nil)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	if w2.Header().Get("X-Next-Called") != "true" {
		t.Error("Expected next handler to be called on success")
	}
}
