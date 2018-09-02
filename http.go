package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RoanBrand/SpectroDashboard/mdb_spectro"
	"github.com/RoanBrand/SpectroDashboard/sample"
	"github.com/RoanBrand/SpectroDashboard/xml_spectro"
)

func setupHTTPServer() {
	http.Handle("/", http.FileServer(http.Dir(filepath.Join(filepath.Dir(execPath), "static"))))
	http.HandleFunc("/results", resultEndpoint)
}

func startHTTPServer(port string) error {
	log.Println("Starting SpectroDashboard service")
	return http.ListenAndServe(":"+port, nil)
}

func resultEndpoint(w http.ResponseWriter, r *http.Request) {
	totalMachineResults := 0
	for _, m := range conf.Machines {
		totalMachineResults = totalMachineResults + m.SampleLimit
	}

	results := make([]sample.Record, 0, totalMachineResults)

	for _, m := range conf.Machines {
		var res []sample.Record
		var err error
		switch strings.ToLower(m.DataType) {
		case "mdb":
			res, err = mdb_spectro.GetResults(m.DataSource, m.SampleLimit, conf.elementOrder)

		case "xml":
			res, err = xml_spectro.GetResults(m.DataSource, m.SampleLimit, conf.elementOrder)
		}
		if err != nil {
			errMsg := "Error querying results: " + err.Error()
			log.Println(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
		results = append(results, res...)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].TimeStamp.After(results[j].TimeStamp)
	})

	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	size := conf.NumberOfResults
	if len(results) < size {
		size = len(results)
	}
	err := enc.Encode(results[:size])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
