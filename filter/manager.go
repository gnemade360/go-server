package filter

import (
	"net/http"
	"sort"
)

// FilterManager embeds filter logic.
type FilterManager struct {
	filters []Filter
}

// AddFilter adds one or more filters to the manager.
func (f *FilterManager) AddFilter(fl ...Filter) {
	f.filters = append(f.filters, fl...)
}

// AddFilterFn adds a filter function with the specified order to the manager.
func (f *FilterManager) AddFilterFn(order int, fn FilterFn) {
	f.filters = append(f.filters, NewFilterFn(order, fn))
}

// ApplyFilters applies all filters in order to the given handler.
func (f *FilterManager) ApplyFilters(h http.Handler) http.Handler {
	sort.Slice(f.filters, func(i, j int) bool { return f.filters[i].Order() < f.filters[j].Order() })
	for i := len(f.filters) - 1; i >= 0; i-- {
		h = Adapter(f.filters[i])(h)
	}
	return h
}
