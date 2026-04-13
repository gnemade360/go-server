package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

// Metric represents a single metric
type Metric struct {
	Name        string                 `json:"name"`
	Type        MetricType             `json:"type"`
	Value       float64                `json:"value"`
	Labels      map[string]string      `json:"labels,omitempty"`
	Help        string                 `json:"help,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// Counter represents a monotonically increasing counter
type Counter struct {
	name   string
	labels map[string]string
	help   string
	value  float64
	mutex  sync.RWMutex
}

// Gauge represents a value that can go up and down
type Gauge struct {
	name   string
	labels map[string]string
	help   string
	value  float64
	mutex  sync.RWMutex
}

// Histogram represents a distribution of values
type Histogram struct {
	name      string
	labels    map[string]string
	help      string
	buckets   []float64
	counts    []uint64
	sum       float64
	count     uint64
	mutex     sync.RWMutex
}

// Registry manages all metrics
type Registry struct {
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
	mutex      sync.RWMutex
}

// NewRegistry creates a new metrics registry
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// Counter methods
func (c *Counter) Inc() {
	c.Add(1)
}

func (c *Counter) Add(value float64) {
	if value < 0 {
		return // Counters can only increase
	}
	c.mutex.Lock()
	c.value += value
	c.mutex.Unlock()
}

func (c *Counter) Get() float64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

func (c *Counter) ToMetric() *Metric {
	return &Metric{
		Name:      c.name,
		Type:      MetricTypeCounter,
		Value:     c.Get(),
		Labels:    c.labels,
		Help:      c.help,
		Timestamp: time.Now(),
	}
}

// Gauge methods
func (g *Gauge) Set(value float64) {
	g.mutex.Lock()
	g.value = value
	g.mutex.Unlock()
}

func (g *Gauge) Inc() {
	g.Add(1)
}

func (g *Gauge) Dec() {
	g.Add(-1)
}

func (g *Gauge) Add(value float64) {
	g.mutex.Lock()
	g.value += value
	g.mutex.Unlock()
}

func (g *Gauge) Get() float64 {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.value
}

func (g *Gauge) ToMetric() *Metric {
	return &Metric{
		Name:      g.name,
		Type:      MetricTypeGauge,
		Value:     g.Get(),
		Labels:    g.labels,
		Help:      g.help,
		Timestamp: time.Now(),
	}
}

// Histogram methods
func (h *Histogram) Observe(value float64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	h.sum += value
	h.count++
	
	// Find the appropriate bucket
	for i, bound := range h.buckets {
		if value <= bound {
			h.counts[i]++
		}
	}
}

func (h *Histogram) GetCount() uint64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.count
}

func (h *Histogram) GetSum() float64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.sum
}

func (h *Histogram) GetMean() float64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

func (h *Histogram) ToMetric() *Metric {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	
	extra := map[string]interface{}{
		"count":   h.count,
		"sum":     h.sum,
		"buckets": h.buckets,
		"counts":  h.counts,
	}
	
	if h.count > 0 {
		extra["mean"] = h.sum / float64(h.count)
	}
	
	return &Metric{
		Name:      h.name,
		Type:      MetricTypeHistogram,
		Value:     h.GetMean(),
		Labels:    h.labels,
		Help:      h.help,
		Timestamp: time.Now(),
		Extra:     extra,
	}
}

// Registry methods
func (r *Registry) NewCounter(name, help string, labels map[string]string) *Counter {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	key := r.getKey(name, labels)
	if counter, exists := r.counters[key]; exists {
		return counter
	}
	
	counter := &Counter{
		name:   name,
		labels: labels,
		help:   help,
		value:  0,
	}
	r.counters[key] = counter
	return counter
}

func (r *Registry) NewGauge(name, help string, labels map[string]string) *Gauge {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	key := r.getKey(name, labels)
	if gauge, exists := r.gauges[key]; exists {
		return gauge
	}
	
	gauge := &Gauge{
		name:   name,
		labels: labels,
		help:   help,
		value:  0,
	}
	r.gauges[key] = gauge
	return gauge
}

func (r *Registry) NewHistogram(name, help string, labels map[string]string, buckets []float64) *Histogram {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	key := r.getKey(name, labels)
	if histogram, exists := r.histograms[key]; exists {
		return histogram
	}
	
	// Ensure buckets are sorted
	sort.Float64s(buckets)
	
	histogram := &Histogram{
		name:    name,
		labels:  labels,
		help:    help,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)),
		sum:     0,
		count:   0,
	}
	r.histograms[key] = histogram
	return histogram
}

func (r *Registry) GetAllMetrics() []*Metric {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var metrics []*Metric
	
	for _, counter := range r.counters {
		metrics = append(metrics, counter.ToMetric())
	}
	
	for _, gauge := range r.gauges {
		metrics = append(metrics, gauge.ToMetric())
	}
	
	for _, histogram := range r.histograms {
		metrics = append(metrics, histogram.ToMetric())
	}
	
	// Sort by name for consistent output
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})
	
	return metrics
}

func (r *Registry) GetMetricsByType(metricType MetricType) []*Metric {
	all := r.GetAllMetrics()
	var filtered []*Metric
	
	for _, metric := range all {
		if metric.Type == metricType {
			filtered = append(filtered, metric)
		}
	}
	
	return filtered
}

func (r *Registry) Reset() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.counters = make(map[string]*Counter)
	r.gauges = make(map[string]*Gauge)
	r.histograms = make(map[string]*Histogram)
}

func (r *Registry) getKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	
	// Create a consistent key by sorting label keys
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	key := name
	for _, k := range keys {
		key += fmt.Sprintf("_%s_%s", k, labels[k])
	}
	
	return key
}

// Handler returns an HTTP handler that serves metrics in JSON format
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		metrics := r.GetAllMetrics()
		response := map[string]interface{}{
			"timestamp": time.Now(),
			"metrics":   metrics,
			"count":     len(metrics),
		}
		
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
			return
		}
	}
}

// PrometheusHandler returns an HTTP handler that serves metrics in Prometheus format
func (r *Registry) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		
		metrics := r.GetAllMetrics()
		
		for _, metric := range metrics {
			// Write help text
			if metric.Help != "" {
				fmt.Fprintf(w, "# HELP %s %s\n", metric.Name, metric.Help)
			}
			
			// Write type
			fmt.Fprintf(w, "# TYPE %s %s\n", metric.Name, string(metric.Type))
			
			// Write metric value
			labels := r.formatLabels(metric.Labels)
			if metric.Type == MetricTypeHistogram {
				// For histograms, write bucket counts, sum, and count
				if extra, ok := metric.Extra["counts"].([]uint64); ok {
					buckets := metric.Extra["buckets"].([]float64)
					for i, bucket := range buckets {
						bucketLabels := r.addLabel(metric.Labels, "le", fmt.Sprintf("%g", bucket))
						fmt.Fprintf(w, "%s_bucket%s %d\n", metric.Name, r.formatLabels(bucketLabels), extra[i])
					}
				}
				fmt.Fprintf(w, "%s_sum%s %g\n", metric.Name, labels, metric.Extra["sum"])
				fmt.Fprintf(w, "%s_count%s %d\n", metric.Name, labels, metric.Extra["count"])
			} else {
				fmt.Fprintf(w, "%s%s %g\n", metric.Name, labels, metric.Value)
			}
		}
	}
}

func (r *Registry) formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	
	var pairs []string
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, k, labels[k]))
	}
	
	return fmt.Sprintf("{%s}", fmt.Sprintf("%s", pairs[0]))
}

func (r *Registry) addLabel(labels map[string]string, key, value string) map[string]string {
	newLabels := make(map[string]string)
	for k, v := range labels {
		newLabels[k] = v
	}
	newLabels[key] = value
	return newLabels
}

// Default registry instance
var defaultRegistry = NewRegistry()

// Package-level convenience functions
func NewCounter(name, help string, labels map[string]string) *Counter {
	return defaultRegistry.NewCounter(name, help, labels)
}

func NewGauge(name, help string, labels map[string]string) *Gauge {
	return defaultRegistry.NewGauge(name, help, labels)
}

func NewHistogram(name, help string, labels map[string]string, buckets []float64) *Histogram {
	return defaultRegistry.NewHistogram(name, help, labels, buckets)
}

func GetAllMetrics() []*Metric {
	return defaultRegistry.GetAllMetrics()
}

func Handler() http.HandlerFunc {
	return defaultRegistry.Handler()
}

func PrometheusHandler() http.HandlerFunc {
	return defaultRegistry.PrometheusHandler()
}

func Reset() {
	defaultRegistry.Reset()
}