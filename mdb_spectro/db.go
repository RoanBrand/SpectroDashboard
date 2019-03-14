package mdb_spectro

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RoanBrand/SpectroDashboard/sample"
	_ "github.com/mattn/go-adodb"
)

// Driver has problems with multiple connections.
// DB is a file on disk anyway.
var querySerializer sync.Mutex

type record struct {
	SampleId   int64
	SampleName string
	Furnace    string
	MeasureId  int64
	TimeStamp  time.Time
	Results    []elementResult
}

type elementResult struct {
	Element string
	Value   float64
}

// Database to actual.
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

func GetLastFurnaceResults(dsn string, furnaces []string, tSamplesOnly bool) ([]sample.Record, error) {
	querySerializer.Lock()
	defer querySerializer.Unlock()

	db, err := sql.Open("adodb", dsn)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %v", err)
	}
	defer db.Close()

	qry := strings.Builder{}
	for _, f := range furnaces {
		qry.WriteString(`
			(SELECT TOP 1 SampleName, Quality, StoreDateTime
			FROM KSampleResultTbl WHERE UCASE(Quality) = '` + strings.ToUpper(f) + `'`)
		if tSamplesOnly {
			qry.WriteString(` AND UCASE(Right(SampleName,1)) = 'T' `)
		}
		qry.WriteString(` ORDER BY SampleResultID DESC) UNION `)
	}

	qryStr := qry.String()[:qry.Len()-7]
	sampleRows, err := db.Query(qryStr)
	if err != nil {
		return nil, fmt.Errorf("error querying 'KSampleResultTbl': %v", err)
	}
	defer sampleRows.Close()

	recs := make([]sample.Record, len(furnaces))

	i := 0
	for sampleRows.Next() {
		err := sampleRows.Scan(&recs[i].SampleName, &recs[i].Furnace, &recs[i].TimeStamp)
		if err != nil {
			return nil, fmt.Errorf("error scanning row from 'KSampleResultTbl': %v", err)
		}

		i++
	}

	return recs[:i], nil
}

func GetResults(dsn string, numResults int, elementsOrder map[string]int) ([]sample.Record, error) {
	querySerializer.Lock()
	defer querySerializer.Unlock()

	db, err := sql.Open("adodb", dsn)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %v", err)
	}
	defer db.Close()

	sampleRows, err := db.Query(`
		SELECT TOP ` + strconv.Itoa(numResults) + ` 
		SampleResultID, SampleName, Quality
		FROM KSampleResultTbl
		ORDER BY SampleResultID DESC;`)
	if err != nil {
		return nil, fmt.Errorf("error querying 'KSampleResultTbl': %v", err)
	}
	defer sampleRows.Close()

	recs := make([]record, 0, numResults)

	for sampleRows.Next() {
		var sampleName sql.NullString
		var furnace sql.NullString
		rec := record{}

		err := sampleRows.Scan(&rec.SampleId, &sampleName, &furnace)
		if err != nil {
			return nil, fmt.Errorf("error scanning row from 'KSampleResultTbl': %v", err)
		}

		if sampleName.Valid {
			rec.SampleName = sampleName.String
		}
		if furnace.Valid {
			rec.Furnace = furnace.String
		}

		recs = append(recs, rec)
	}

	for i := range recs {
		rec := &recs[i]
		measureResultRows, err := db.Query(`
			SELECT
			m.MeasureResultID, m.Timestamp, r.ResultKey, r.Value
			FROM KMeasureResultTbl m
			LEFT JOIN KResultValueTbl r ON ((r.MeasureResultID = m.MeasureResultID) AND (r.ResultType = 2) AND (r.Value > 0.0))
			WHERE m.SampleResultID = ` + strconv.FormatInt(rec.SampleId, 10) + ` AND m.ResultType = 1;`)
		if err != nil {
			return nil, fmt.Errorf("error querying 'KMeasureResultTbl': %v", err)
		}

		rec.Results = make([]elementResult, len(elementsOrder))

		for measureResultRows.Next() {
			var elCode sql.NullString
			var elValue sql.NullFloat64

			err := measureResultRows.Scan(&rec.MeasureId, &rec.TimeStamp, &elCode, &elValue)
			if err != nil {
				measureResultRows.Close()
				return nil, fmt.Errorf("error scanning row from 'KMeasureResultTbl': %v", err)
			}

			if !elCode.Valid || !elValue.Valid {
				continue
			}

			// lookup element from db element value
			if el, ok := elementMap[elCode.String]; ok {
				order := elementsOrder[el]
				res := &rec.Results[order]
				if res.Element == "" { // wouldn't this be always blank?
					res.Element = el
					res.Value = elValue.Float64
				}
			}
		}
		measureResultRows.Close()
	}

	results := make([]sample.Record, len(recs))

	// turn recs into results
	for i := range recs {
		results[i].SampleName = recs[i].SampleName
		results[i].Furnace = recs[i].Furnace
		results[i].TimeStamp = recs[i].TimeStamp
		results[i].Results = make([]sample.ElementResult, len(recs[i].Results))
		for j := range recs[i].Results {
			results[i].Results[j].Element = recs[i].Results[j].Element
			results[i].Results[j].Value = recs[i].Results[j].Value
		}
	}

	return results, nil
}
