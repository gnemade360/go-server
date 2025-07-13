package filter

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestFilterManager_AddFilter(t *testing.T) {
	manager := &FilterManager{}

	filter1 := &mockFilter{order: 1, headerKey: "X-Filter-1", value: "first"}
	filter2 := &mockFilter{order: 2, headerKey: "X-Filter-2", value: "second"}

	// Test adding single filter
	manager.AddFilter(filter1)

	if len(manager.filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(manager.filters))
	}

	// Test adding multiple filters at once
	filter3 := &mockFilter{order: 3, headerKey: "X-Filter-3", value: "third"}
	manager.AddFilter(filter2, filter3)

	if len(manager.filters) != 3 {
		t.Errorf("Expected 3 filters, got %d", len(manager.filters))
	}

	// Verify filters were added correctly
	if manager.filters[0] != filter1 {
		t.Error("First filter was not added correctly")
	}
	if manager.filters[1] != filter2 {
		t.Error("Second filter was not added correctly")
	}
	if manager.filters[2] != filter3 {
		t.Error("Third filter was not added correctly")
	}
}

func TestFilterManager_AddFilterFn(t *testing.T) {
	manager := &FilterManager{}

	executed1 := false
	fn1 := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executed1 = true
		w.Header().Set("X-Fn-1", "first")
		next.ServeHTTP(w, r)
	}

	executed2 := false
	fn2 := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executed2 = true
		w.Header().Set("X-Fn-2", "second")
		next.ServeHTTP(w, r)
	}

	manager.AddFilterFn(5, fn1)
	manager.AddFilterFn(3, fn2)

	if len(manager.filters) != 2 {
		t.Errorf("Expected 2 filters, got %d", len(manager.filters))
	}

	// Test that filters were added with correct order
	if manager.filters[0].Order() != 5 {
		t.Errorf("Expected first filter order 5, got %d", manager.filters[0].Order())
	}
	if manager.filters[1].Order() != 3 {
		t.Errorf("Expected second filter order 3, got %d", manager.filters[1].Order())
	}

	// Test that the filters work
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	manager.filters[0].Do(w, req, nextHandler)
	if !executed1 {
		t.Error("First filter function was not executed")
	}

	w = httptest.NewRecorder() // Reset recorder
	manager.filters[1].Do(w, req, nextHandler)
	if !executed2 {
		t.Error("Second filter function was not executed")
	}
}

func TestFilterManager_ApplyFilters_EmptyManager(t *testing.T) {
	manager := &FilterManager{}

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original", "called")
		w.WriteHeader(http.StatusOK)
	})

	// Apply filters to empty manager should return original handler
	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	resultHandler.ServeHTTP(w, req)

	if w.Header().Get("X-Original") != "called" {
		t.Error("Expected original handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestFilterManager_ApplyFilters_SingleFilter(t *testing.T) {
	manager := &FilterManager{}

	filter := &mockFilter{order: 1, headerKey: "X-Single", value: "applied"}
	manager.AddFilter(filter)

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original", "called")
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	resultHandler.ServeHTTP(w, req)

	if !filter.executed {
		t.Error("Filter was not executed")
	}

	if w.Header().Get("X-Single") != "applied" {
		t.Error("Expected filter header to be set")
	}

	if w.Header().Get("X-Original") != "called" {
		t.Error("Expected original handler to be called")
	}
}

func TestFilterManager_ApplyFilters_MultipleFilters_Ordered(t *testing.T) {
	manager := &FilterManager{}

	// Add filters in non-sequential order to test sorting
	filter2 := &mockFilter{order: 2, headerKey: "X-Order", value: "2"}
	filter1 := &mockFilter{order: 1, headerKey: "X-Order", value: "1"}
	filter3 := &mockFilter{order: 3, headerKey: "X-Order", value: "3"}

	manager.AddFilter(filter2, filter1, filter3)

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original", "called")
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	resultHandler.ServeHTTP(w, req)

	// All filters should be executed
	if !filter1.executed {
		t.Error("Filter 1 was not executed")
	}
	if !filter2.executed {
		t.Error("Filter 2 was not executed")
	}
	if !filter3.executed {
		t.Error("Filter 3 was not executed")
	}

	// The last filter to execute should have its header value (filter 3)
	// because each filter overwrites the X-Order header
	if w.Header().Get("X-Order") != "3" {
		t.Errorf("Expected final header value '3', got %s", w.Header().Get("X-Order"))
	}

	if w.Header().Get("X-Original") != "called" {
		t.Error("Expected original handler to be called")
	}
}

func TestFilterManager_ApplyFilters_ExecutionOrder(t *testing.T) {
	manager := &FilterManager{}

	var executionOrder []string

	// Create filters that record execution order
	filter1 := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executionOrder = append(executionOrder, "filter1-start")
		w.Header().Set("X-Filter-1", "executed")
		next.ServeHTTP(w, r)
		executionOrder = append(executionOrder, "filter1-end")
	})

	filter3 := NewFilterFn(3, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executionOrder = append(executionOrder, "filter3-start")
		w.Header().Set("X-Filter-3", "executed")
		next.ServeHTTP(w, r)
		executionOrder = append(executionOrder, "filter3-end")
	})

	filter2 := NewFilterFn(2, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executionOrder = append(executionOrder, "filter2-start")
		w.Header().Set("X-Filter-2", "executed")
		next.ServeHTTP(w, r)
		executionOrder = append(executionOrder, "filter2-end")
	})

	// Add filters in non-sequential order
	manager.AddFilter(filter3, filter1, filter2)

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder = append(executionOrder, "original-handler")
		w.Header().Set("X-Original", "called")
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	resultHandler.ServeHTTP(w, req)

	// Expected order: filter1-start, filter2-start, filter3-start, original-handler, filter3-end, filter2-end, filter1-end
	expectedOrder := []string{
		"filter1-start", "filter2-start", "filter3-start",
		"original-handler",
		"filter3-end", "filter2-end", "filter1-end",
	}

	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d execution steps, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if i >= len(executionOrder) || executionOrder[i] != expected {
			t.Errorf("Expected execution step %d to be '%s', got '%s'", i, expected, getStringOrEmpty(executionOrder, i))
		}
	}
}

func getStringOrEmpty(slice []string, index int) string {
	if index < len(slice) {
		return slice[index]
	}
	return "<missing>"
}

func TestFilterManager_ApplyFilters_ConditionalFilters(t *testing.T) {
	manager := &FilterManager{}

	// Create conditional filter that only applies to /special path
	conditionalFilter := &mockConditionalFilter{
		mockFilter: mockFilter{
			order:     1,
			headerKey: "X-Conditional",
			value:     "applied",
		},
		matchCondition: func(r *http.Request) bool {
			return r.URL.Path == "/special"
		},
	}

	// Create regular filter that always applies
	regularFilter := &mockFilter{order: 2, headerKey: "X-Regular", value: "always"}

	manager.AddFilter(conditionalFilter, regularFilter)

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original", "called")
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	// Test with matching path
	req1 := httptest.NewRequest("GET", "/special", nil)
	w1 := httptest.NewRecorder()

	resultHandler.ServeHTTP(w1, req1)

	if !conditionalFilter.executed {
		t.Error("Conditional filter should have been executed for /special path")
	}

	if !regularFilter.executed {
		t.Error("Regular filter should have been executed")
	}

	if w1.Header().Get("X-Conditional") != "applied" {
		t.Error("Expected conditional filter header")
	}

	if w1.Header().Get("X-Regular") != "always" {
		t.Error("Expected regular filter header")
	}

	// Reset filters for next test
	conditionalFilter.executed = false
	regularFilter.executed = false

	// Test with non-matching path
	req2 := httptest.NewRequest("GET", "/other", nil)
	w2 := httptest.NewRecorder()

	resultHandler.ServeHTTP(w2, req2)

	if conditionalFilter.executed {
		t.Error("Conditional filter should not have been executed for /other path")
	}

	if !regularFilter.executed {
		t.Error("Regular filter should have been executed")
	}

	if w2.Header().Get("X-Conditional") != "" {
		t.Error("Expected no conditional filter header")
	}

	if w2.Header().Get("X-Regular") != "always" {
		t.Error("Expected regular filter header")
	}
}

func TestFilterManager_ApplyFilters_SortingStability(t *testing.T) {
	manager := &FilterManager{}

	// Create multiple filters with the same order to test stable sorting
	var executionOrder []string

	filterA := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executionOrder = append(executionOrder, "A")
		next.ServeHTTP(w, r)
	})

	filterB := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executionOrder = append(executionOrder, "B")
		next.ServeHTTP(w, r)
	})

	filterC := NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executionOrder = append(executionOrder, "C")
		next.ServeHTTP(w, r)
	})

	// Add in specific order
	manager.AddFilter(filterA, filterB, filterC)

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	resultHandler.ServeHTTP(w, req)

	// Since all filters have the same order, they should execute in the order they were added
	expectedOrder := []string{"A", "B", "C"}

	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d filters to execute, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if i >= len(executionOrder) || executionOrder[i] != expected {
			t.Errorf("Expected filter %d to be '%s', got '%s'", i, expected, getStringOrEmpty(executionOrder, i))
		}
	}
}

func TestFilterManager_ApplyFilters_MixedFilterFnAndRegular(t *testing.T) {
	manager := &FilterManager{}

	// Add regular filter
	regularFilter := &mockFilter{order: 2, headerKey: "X-Regular", value: "regular"}
	manager.AddFilter(regularFilter)

	// Add filter function
	executed := false
	manager.AddFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		executed = true
		w.Header().Set("X-FilterFn", "fn")
		next.ServeHTTP(w, r)
	})

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Original", "called")
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	resultHandler.ServeHTTP(w, req)

	if !executed {
		t.Error("FilterFn was not executed")
	}

	if !regularFilter.executed {
		t.Error("Regular filter was not executed")
	}

	if w.Header().Get("X-FilterFn") != "fn" {
		t.Error("Expected FilterFn header")
	}

	if w.Header().Get("X-Regular") != "regular" {
		t.Error("Expected regular filter header")
	}

	if w.Header().Get("X-Original") != "called" {
		t.Error("Expected original handler to be called")
	}
}

func BenchmarkFilterManager_ApplyFilters(b *testing.B) {
	manager := &FilterManager{}

	// Add multiple filters
	for i := 0; i < 5; i++ {
		order := i + 1
		manager.AddFilterFn(order, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
			w.Header().Set("X-Filter-"+strconv.Itoa(order), "applied")
			next.ServeHTTP(w, r)
		})
	}

	originalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resultHandler := manager.ApplyFilters(originalHandler)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		resultHandler.ServeHTTP(w, req)
	}
}

func BenchmarkFilterManager_AddFilter(b *testing.B) {
	filter := &mockFilter{order: 1, headerKey: "X-Test", value: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := &FilterManager{}
		manager.AddFilter(filter)
	}
}

func BenchmarkFilterManager_AddFilterFn(b *testing.B) {
	fn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		next.ServeHTTP(w, r)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := &FilterManager{}
		manager.AddFilterFn(1, fn)
	}
}
