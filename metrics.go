package main

type MetricResponse struct {
	Gauges []Gauge `json:"Gauges"`
}

type Gauge struct {
	Name  string `json:"Name"`
	Value int64  `json:"Value"`
}
