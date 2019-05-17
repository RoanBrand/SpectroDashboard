package sample

import "time"

type Record struct {
	SampleName string          `json:"sample_name"`
	Furnace    string          `json:"furnace"`
	TimeStamp  time.Time       `json:"time_stamp"`
	Results    []ElementResult `json:"results,omitempty"`

	Spectro int // spectro machine from which the sample was taken
}

type ElementResult struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
}
