package main

import (
	"database/sql"
	"strconv"
	"sync"
	"time"

	_ "github.com/mattn/go-adodb"
)

// Driver has problems with multiple connections.
// DB is a file on disk anyway.
var querySerializer sync.Mutex

type record struct {
	SampleId   int64            `json:"sample_id"`
	SampleName string           `json:"sample_name"`
	Furnace    string           `json:"furnace"`
	MeasureId  int64            `json:"measure_id"`
	TimeStamp  time.Time        `json:"time_stamp"`
	Results    []*elementResult `json:"results"`
}

type elementResult struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
}

func queryResults(dsn string, numResults int) ([]*record, error) {
	querySerializer.Lock()
	defer querySerializer.Unlock()

	db, err := sql.Open("adodb", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	sampleRows, err := db.Query(`
		SELECT TOP ` + strconv.Itoa(numResults) + ` 
		SampleResultID, SampleName, Quality
		FROM KSampleResultTbl
		ORDER BY SampleResultID DESC;`)
	if err != nil {
		return nil, err
	}
	defer sampleRows.Close()

	recs := make([]*record, 0, numResults)

	for sampleRows.Next() {
		rec := &record{}
		err := sampleRows.Scan(&rec.SampleId, &rec.SampleName, &rec.Furnace)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}

	for _, rec := range recs {
		measureResultRows, err := db.Query(`
			SELECT
			m.MeasureResultID, m.Timestamp, r.ResultKey, r.Value
			FROM KMeasureResultTbl m
			LEFT JOIN KResultValueTbl r ON ((r.MeasureResultID = m.MeasureResultID) AND (r.ResultType = 2) AND (r.Value > 0.0))
			WHERE m.SampleResultID = ` + strconv.FormatInt(rec.SampleId, 10) + ` AND m.ResultType = 1;`)
		if err != nil {
			return nil, err
		}

		rec.Results = make([]*elementResult, len(elementOrder))

		for measureResultRows.Next() {
			var elCode string
			var elValue float64

			err := measureResultRows.Scan(&rec.MeasureId, &rec.TimeStamp, &elCode, &elValue)
			if err != nil {
				measureResultRows.Close()
				return nil, err
			}

			if el, ok := elementMap[elCode]; ok {
				order := elementOrder[el]
				if rec.Results[order] == nil {
					rec.Results[order] = &elementResult{Element: el, Value: elValue}
				}
			}
		}
		measureResultRows.Close()
	}
	return recs, nil
}
