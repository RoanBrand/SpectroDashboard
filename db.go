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

/*
var db *sql.DB

func startDBConn(dsn string) error {
	var err error
	db, err = sql.Open("adodb", dsn)
	if err != nil {
		return err
	}

	return nil
}
*/
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

	recs := make([]*record, 0)

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
			SELECT TOP 1
			MeasureResultID, Timestamp
			FROM KMeasureResultTbl
			WHERE SampleResultID = ` + strconv.FormatInt(rec.SampleId, 10) + ` AND ResultType = 1;`)
		if err != nil {
			return nil, err
		}

		for measureResultRows.Next() {
			err := measureResultRows.Scan(&rec.MeasureId, &rec.TimeStamp)
			if err != nil {
				measureResultRows.Close()
				return nil, err
			}
		}
		measureResultRows.Close()
	}

	for _, rec := range recs {
		resultValueRows, err := db.Query(`
			SELECT
			ResultKey, Value
			FROM KResultValueTbl
			WHERE MeasureResultID = ` + strconv.FormatInt(rec.MeasureId, 10) + `AND ResultType = 2`)
		if err != nil {
			return nil, err
		}
		rec.Results = make([]*elementResult, len(elementOrder))

		for resultValueRows.Next() {
			var elCode string
			var elValue float64

			err := resultValueRows.Scan(&elCode, &elValue)
			if err != nil {
				resultValueRows.Close()
				return nil, err
			}
			if elValue == 0.0 {
				continue
			}
			if el, ok := elementMap[elCode]; ok {
				order := elementOrder[el]
				if rec.Results[order] == nil {
					rec.Results[order] = &elementResult{Element: el, Value: elValue}
				}
			}
		}
		resultValueRows.Close()
	}
	return recs, nil
}
