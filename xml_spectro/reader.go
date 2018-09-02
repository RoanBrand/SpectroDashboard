package xml_spectro

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroDashboard/sample"
)

// GetResults(dsn string, numResults int, elementsOrder map[string]int) ([]sample.Record, error) {
func GetResults(xmlFolder string, numResults int, elementsOrder map[string]int) ([]sample.Record, error) {
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

	recs := make([]sample.Record, numResults)

	for i, srXML := range results {
		for _, sr := range srXML.SampleResults {
			recs[i].SampleName = sr.SampleID()
			recs[i].Furnace = sr.Furnace()
			recs[i].TimeStamp, err = time.Parse("2006-01-02T15:04:05", sr.Timestamp)
			if err != nil {
				continue
			}
			recs[i].Results = make([]sample.ElementResult, len(elementsOrder))
			totalElements := 0
			for _, el := range sr.MeasurementStatistics[0].Elements {
				res := el.reportedResult()
				if res == nil {
					continue
				}

				// lookup element. if not present it is not one we want
				if order, present := elementsOrder[el.Name]; present {
					recs[i].Results[order].Element = el.Name
					recs[i].Results[order].Value = res.ResultValue
					totalElements++
				}
				if totalElements == len(elementsOrder) {
					break
				}
			}
		}
	}

	/*for _, srXML := range results {
		for _, sr := range srXML.SampleResults {
			fmt.Printf("Time: %s, SampleID: %s, Furnace: %s, Operator: %s\n", sr.Timestamp, sr.SampleID(), sr.Furnace(), sr.Operator())
			fmt.Println("Results:")
			for _, el := range sr.MeasurementStatistics[0].Elements {
				res := el.reportedResult()
				if res == nil {
					fmt.Println(el.Name, "shit")
					continue
				}
				fmt.Println(el.Name, res.ResultValue)
			}
			fmt.Println()
		}
	}*/

	return recs, nil
}
