package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
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
	results, err := queryResults(conf.DataSource, conf.NumberOfResults)
	if err != nil {
		errMsg := "Error querying results: " + err.Error()
		log.Println(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	err = enc.Encode(results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
