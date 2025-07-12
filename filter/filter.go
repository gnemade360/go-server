package filter

import "net/http"

// Functional shortcut -------------------------------------------------

type FilterFn func(http.ResponseWriter, *http.Request, http.Handler)

// Core interface ------------------------------------------------------

type Filter interface {
	Order() int
	Do(http.ResponseWriter, *http.Request, http.Handler)
}

type ConditionalFilter interface {
	Filter
	Match(*http.Request) bool
}

// Constructor ---------------------------------------------------------

type fnFilter struct {
	order int
	fn    FilterFn
}

func (f fnFilter) Order() int                                                   { return f.order }
func (f fnFilter) Do(w http.ResponseWriter, r *http.Request, next http.Handler) { f.fn(w, r, next) }
func NewFilterFn(order int, fn FilterFn) Filter                                 { return fnFilter{order: order, fn: fn} }

// Adapter turns Filter into middleware.
func Adapter(f Filter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if cf, ok := f.(ConditionalFilter); ok && !cf.Match(r) {
				next.ServeHTTP(w, r)
				return
			}
			f.Do(w, r, next)
		})
	}
}
