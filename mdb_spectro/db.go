package mdb_spectro

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/RoanBrand/SpectroDashboard/sample"
	_ "github.com/mattn/go-adodb"
)

// Driver has problems with multiple connections.
// DB is a file on disk anyway.
var querySerializer sync.Mutex

/*type record struct {
	SampleId   int64
	SampleName string
	Furnace    string
	MeasureId  int64
	TimeStamp  time.Time
	Results    map[string]float64
}*/

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
	"0x0000000E-Ni": "Ni",
	"0x00000011-Mo": "Mo",
	"0x00000017-Co": "Co",
	"0x0000001D-Nb": "Nb",
	"0x00000021-V":  "V",
	"0x00000023-W":  "W",
	"0x00000029-Mg": "Mg",
	// "": "As",
	"0x0000002B-Bi": "Bi",
	"0x0000002D-Ca": "Ca",
	// "": "Sb",
	// "": "Te",
	"0x00000033-Fe": "Fe",
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

func GetResults(dsn string, numResults int) ([]*sample.Record, error) {
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

	recs := make([]*sample.Record, 0, numResults)

	for sampleRows.Next() {
		var sampleName sql.NullString
		var furnace sql.NullString
		r := new(sample.Record)

		err := sampleRows.Scan(&r.SampleId, &sampleName, &furnace)
		if err != nil {
			return nil, fmt.Errorf("error scanning row from 'KSampleResultTbl': %v", err)
		}

		if sampleName.Valid {
			r.SampleName = sampleName.String
		}
		if furnace.Valid {
			r.Furnace = furnace.String
		}

		recs = append(recs, r)
	}

	for _, r := range recs {
		measureResultRows, err := db.Query(`
			SELECT m.Timestamp, r.ResultKey, r.Value
			FROM KMeasureResultTbl m
			LEFT JOIN KResultValueTbl r ON ((r.MeasureResultID = m.MeasureResultID) AND (r.ResultType = 2) AND (r.Value > 0.0))
			WHERE m.SampleResultID = ` + strconv.FormatInt(r.SampleId, 10) + ` AND m.ResultType = 1;`)
		if err != nil {
			return nil, fmt.Errorf("error querying 'KMeasureResultTbl': %v", err)
		}

		r.ResultsMap = make(map[string]float64, len(elementMap))

		for measureResultRows.Next() {
			var elCode sql.NullString
			var elValue sql.NullFloat64

			err := measureResultRows.Scan(&r.TimeStamp, &elCode, &elValue)
			if err != nil {
				measureResultRows.Close()
				return nil, fmt.Errorf("error scanning row from 'KMeasureResultTbl': %v", err)
			}

			if !elCode.Valid || !elValue.Valid {
				continue
			}

			if el, ok := elementMap[elCode.String]; ok {
				if _, ok := r.ResultsMap[el]; !ok {
					r.ResultsMap[el] = elValue.Float64
				}
			}
		}
		measureResultRows.Close()
	}

	return recs, nil
}
