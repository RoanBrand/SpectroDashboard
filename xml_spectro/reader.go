package xml_spectro

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var elements = map[string]struct{}{
	"C":  {},
	"Si": {},
	"Mn": {},
	"P":  {},
	"S":  {},
	"Cu": {},
	"Cr": {},
	"Al": {},
	"Ti": {},
	"Sn": {},
	"Zn": {},
	"Pb": {},
	"Ni": {},
	"Mo": {},
	"Co": {},
	"Nb": {},
	"V":  {},
	"W":  {},
	"Mg": {},
	"As": {},
	"Bi": {},
	"Ca": {},
	"Sb": {},
	"Te": {},
}

type record struct {
	ID        string             `json:"id"`
	Furnace   string             `json:"furnace"`
	TimeStamp time.Time          `json:"time_stamp"`
	Results   map[string]float64 `json:"results"`
}

// GetResults(dsn string, numResults int, elementsOrder map[string]int) ([]sample.Record, error) {
func GetResults(xmlFolder string, numResults int) ([]record, error) {
	files, err := filepath.Glob(filepath.Join(xmlFolder, "*"))
	if err != nil {
		return nil, err
	}

	// sort files (spectro XML file names contain dates, so we assume results will be sorted)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	// filter out any non spectro result xml files.
	xmlFiles := files[:0]
	for _, file := range files {
		if filepath.Ext(file) == ".xml" && strings.Contains(file, "spectro") {
			xmlFiles = append(xmlFiles, file)
		}
	}

	if len(xmlFiles) < numResults {
		numResults = len(xmlFiles)
	}

	results := make([]SampleResultsXMLFile, numResults)

	for i := 0; i < numResults; i++ {
		f, err := os.Open(xmlFiles[i])
		if err != nil {
			return nil, err
		}

		dec := xml.NewDecoder(f)
		err = dec.Decode(&results[i])
		if err != nil {
			return nil, err
		}
	}

	recs := make([]record, numResults)

	for i, srXML := range results {
		for _, sr := range srXML.SampleResults { // is actually one sample per file
			rec := &recs[i]
			rec.ID = sr.SampleID()
			rec.Furnace = sr.Furnace()
			rec.TimeStamp, err = time.ParseInLocation("2006-01-02T15:04:05", sr.Timestamp, time.Local)
			if err != nil {
				continue
			}

			rec.Results = make(map[string]float64, len(elements))
			totalElements := 0
			for _, el := range sr.MeasurementStatistics[0].Elements {
				res := el.reportedResult()
				if res == nil {
					continue
				}

				// lookup element. if not present it is not one we want
				if _, present := elements[el.Name]; present {
					rec.Results[el.Name] = res.ResultValue
					totalElements++
				}
				if totalElements == len(elements) {
					break
				}
			}
		}
	}

	return recs, nil
}
