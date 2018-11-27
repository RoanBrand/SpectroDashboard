package sample

import "time"

type Record struct {
	SampleName string          `json:"sample_name"`
	Furnace    string          `json:"furnace"`
	TimeStamp  time.Time       `json:"time_stamp"`
	Results    []ElementResult `json:"results,omitempty"`
}

type ElementResult struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
}
