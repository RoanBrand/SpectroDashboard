package main

import (
	"database/sql"
	"strconv"
	"time"
)

var db *sql.DB

func startDBConn(dsn string) error {
	var err error
	db, err = sql.Open("adodb", dsn)
	if err != nil {
		return err
	}

	return nil
}

type record struct {
	SampleId   int64           `json:"sample_id"`
	SampleName string          `json:"sample_name"`
	Furnace    string          `json:"furnace"`
	MeasureId  int64           `json:"measure_id"`
	TimeStamp  time.Time       `json:"time_stamp"`
	Results    []elementResult `json:"results"`
}

type elementResult struct {
	Element string  `json:"element"`
	Value   float64 `json:"value"`
}

var elementMap = map[string]string{
	"0x00000001-C":  "C",
	"0x00000003-Si": "Si",
	"0x00000005-Mn": "Mn",
	"0x00000007-P":  "P",
	"0x00000009-S":  "S",
	"0x00000019-Cu": "Cu",
	"0x0000000B-Cr": "Cr",
	"0x00000015-Al": "Al",
	"0x0000001F-Ti": "Ti",
	"0x00000027-Sn": "Sn",
	"0x00000031-Zn": "Zn",
	"0x00000025-Pb": "Pb",
}

func queryResults() ([]*record, error) {
	sampleRows, err := db.Query(`
		SELECT TOP 20
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
		defer measureResultRows.Close()

		for measureResultRows.Next() {
			err := measureResultRows.Scan(&rec.MeasureId, &rec.TimeStamp)
			if err != nil {
				return nil, err
			}
		}
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
		defer resultValueRows.Close()
		rec.Results = make([]elementResult, 0)
		els := make(map[string]struct{})

		for resultValueRows.Next() {
			var elCode string
			var elValue float64

			err := resultValueRows.Scan(&elCode, &elValue)
			if err != nil {
				return nil, err
			}
			if _, ok := els[elCode]; ok || elValue == 0.0 {
				continue
			}
			if el, ok := elementMap[elCode]; ok {
				els[elCode] = struct{}{}
				rec.Results = append(rec.Results, elementResult{Element: el, Value: elValue})
			}
		}
	}
	return recs, nil
}
