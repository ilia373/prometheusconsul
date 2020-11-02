package counters

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Subscribable implement the subscribable interface
type Subscribable struct {
	method func(labels []string, val interface{})
	active int32

	values sync.Map
}

// Subscribe start getting events
func (s *Subscribable) Subscribe(method func(labels []string, val interface{})) {
	if !atomic.CompareAndSwapInt32(&s.active, 0, 1) {
		panic("support a single subscriber")
	}
	s.method = method
}

func (s *Subscribable) createKey(labels []string) string {
	var sb strings.Builder
	for _, l := range labels {
		sb.WriteString(l)
	}
	return sb.String()
}

// RaiseDelta a new event
func (s *Subscribable) raiseDelta(labels []string, delta float64) {
	active := atomic.LoadInt32(&s.active)
	if active == 1 && s.method != nil {
		key := s.createKey(labels)
		val, ok := s.values.Load(key) // this implementation may produce the wrong result in case the same counter was updated from
		// different goroutines at the same time
		newVal := delta
		if ok {
			newVal = val.(float64) + delta
			s.values.Store(key, newVal)
		} else {
			s.values.Store(key, newVal)
		}
		s.method(labels, newVal)
	}
}

// RaiseValue a new event
func (s *Subscribable) raiseValue(labels []string, val float64) {
	active := atomic.LoadInt32(&s.active)
	if active == 1 && s.method != nil {
		key := s.createKey(labels)
		s.values.Store(key, val)
		s.method(labels, val)
	}
}

// Unsubscribe Stop getting events
func (s *Subscribable) Unsubscribe() {
	atomic.StoreInt32(&s.active, 0)
	s.method = nil
}

// Counter represent a counter that can only go up
type Counter struct {
	counter *prometheus.CounterVec
	active  bool
	vals    []string
	Subscribable
}

// WithData associate data with the counter
func (c *Counter) WithData(lvs ...string) *Counter {
	newVals := append(c.vals, lvs...)
	return &Counter{active: c.active, counter: c.counter, vals: newVals, Subscribable: c.Subscribable}
}

//Inc increment the value by one
func (c *Counter) Inc(lvs ...string) {
	if !c.active {
		return
	}
	values := append(c.vals, lvs...)
	c.counter.WithLabelValues(values...).Inc()
	c.raiseDelta(values, 1.0)
}

//Add to counter the value Val
func (c *Counter) Add(val int, lvs ...string) {
	if !c.active {
		return
	}
	values := append(c.vals, lvs...)
	c.counter.WithLabelValues(values...).Add(float64(val))
	c.raiseDelta(values, float64(val))
}

// Gauge represent a value that you can set to any number
type Gauge struct {
	gauge  *prometheus.GaugeVec
	active bool
	vals   []string
	Subscribable
}

// WithData associate data with the Gauge
func (g *Gauge) WithData(lvs ...string) *Gauge {
	newVals := append(g.vals, lvs...)
	return &Gauge{active: g.active, gauge: g.gauge, vals: newVals, Subscribable: g.Subscribable}
}

//Set set a value
func (g *Gauge) Set(val float64, lvs ...string) {
	if !g.active {
		return
	}
	values := append(g.vals, lvs...)
	g.gauge.WithLabelValues(values...).Set(val)
	g.raiseValue(values, val)
}

//Inc increment the value by one
func (g *Gauge) Inc(lvs ...string) {
	if !g.active {
		return
	}
	values := append(g.vals, lvs...)
	g.gauge.WithLabelValues(values...).Inc()
	g.raiseDelta(values, 1.0)
}

//Dec decrement the value by one
func (g *Gauge) Dec(lvs ...string) {
	if !g.active {
		return
	}
	values := append(g.vals, lvs...)
	g.gauge.WithLabelValues(values...).Dec()
	g.raiseDelta(values, -1.0)
}

// NewCounters entry point to the counters library
func NewCounters(port int, active bool) *Counters {
	return &Counters{port: port, active: active}
}

// Counters type implementing counter logic
type Counters struct {
	s      *http.Server
	port   int
	active bool
}

//CreateCounter create an entity of type counter
func (c *Counters) CreateCounter(name, help string, lables []string) (*Counter, error) {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, lables)
	err := prometheus.Register(counter)
	return &Counter{counter: counter, active: c.active}, err
}

//CreateGauge create an entity of type Gauge
func (c *Counters) CreateGauge(name, help string, lables []string) (*Gauge, error) {
	counter := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, lables)
	err := prometheus.Register(counter)
	return &Gauge{gauge: counter, active: c.active}, err
}

//Start start the prometheus endpoint
func (c *Counters) Start() error {
	addr := fmt.Sprintf(":%d", c.port)

	c.s = &http.Server{
		Addr:           addr,
		ReadTimeout:    8 * time.Second,
		WriteTimeout:   8 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        promhttp.Handler(),
	}

	go c.s.ListenAndServe()
	return nil
}

// AlreadyRegistered returns true if err from the type "AlreadyRegistered" and false otherwise
func (c *Counters) AlreadyRegistered(err error) bool {
	return err.Error() == prometheus.AlreadyRegisteredError{}.Error()
}

//Close Close the prometheus endpoint
func (c *Counters) Close() error {
	if c.s != nil {
		return c.s.Close()
	}
	return nil
}
