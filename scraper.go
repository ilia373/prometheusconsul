package main

import (
	"encoding/json"
	"fmt"
	count "github.com/ilia373/prometheusconsul/counters"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Scraper struct {
	cc       map[string]*count.Gauge
	counters *count.Counters
}

func NewScraper(port int) *Scraper {
	counters := count.NewCounters(port, true)
	return &Scraper{cc: map[string]*count.Gauge{}, counters: counters}
}

func (s *Scraper) Start() error {
	err := s.counters.Start()
	go s.readConsulMetrics()
	return err
}

func (s *Scraper) Close() error {
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
					s.cc[gauge.Name] = s.generateCounter(gauges)
				}
				s.cc[gauge.Name].Set(float64(gauge.Value))
			}
		}
	}
}

func (s *Scraper) generateCounter(gauges []Gauge) *count.Gauge {
	var cc *count.Gauge
	for _, gauge := range gauges {
		c, err := s.counters.CreateGauge(gauge.Name, gauge.Name, []string{"name"})
		if err != nil {
			fmt.Errorf("failed to create counter %s", err)
		} else {
			cc = c.WithData(gauge.Name)
		}
	}
	return cc
}
