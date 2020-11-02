package prometheusconsul

import (
	"encoding/json"
	"fmt"
	count "github.com/ilia373/prometheusconsul/counters"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

//Scraper struct
type Scraper struct {
	cc       map[string]*count.Gauge
	counters *count.Counters
	done     chan bool
}

//NewScraper factory method
func NewScraper(port int) *Scraper {
	counters := count.NewCounters(port, true)
	return &Scraper{cc: map[string]*count.Gauge{}, counters: counters, done: make(chan bool)}
}

func (s *Scraper) Start() error {
	err := s.counters.Start()
	go s.readConsulMetrics()
	return err
}

func (s *Scraper) Close() error {
	s.done <- true
	return s.counters.Close()
}

func (s *Scraper) changeNames(gauges []Gauge) []Gauge {
	var gs []Gauge
	for _, gauge := range gauges {
		gs = append(gs, Gauge{
			Name:  strings.ReplaceAll(gauge.Name, ".", "_"),
			Value: gauge.Value,
		})
	}
	return gs
}

func (s *Scraper) readConsulMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			resp, err := http.Get("http://127.0.0.1:8500/v1/agent/metrics")
			if err != nil {
				fmt.Errorf("err metrics request %v", err)
			}
			res := MetricResponse{}
			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			err = json.Unmarshal(body, &res)
			gauges := s.changeNames(res.Gauges)
			for _, gauge := range gauges {
				if _, ok := s.cc[gauge.Name]; !ok {
					if c, err := s.generateCounter(gauge); err == nil {
						s.cc[gauge.Name] = c
						s.cc[gauge.Name].Set(float64(gauge.Value))
					}
				} else {
					s.cc[gauge.Name].Set(float64(gauge.Value))
				}
			}
		}
	}
}

func (s *Scraper) generateCounter(gauge Gauge) (*count.Gauge, error) {
	var cc *count.Gauge
	c, err := s.counters.CreateGauge(gauge.Name, gauge.Name, []string{"name"})
	if err != nil {
		return nil, fmt.Errorf("failed to create counter %s", err)
	}
	cc = c.WithData(gauge.Name)
	return cc, nil
}
