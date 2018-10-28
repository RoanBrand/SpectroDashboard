package http

import (
	"encoding/json"
	"net/http"

	"github.com/RoanBrand/SpectroDashboard/log"
)

var resultFunc func() (interface{}, error)

func SetupServer(staticFilesPath string) {
	http.Handle("/", http.FileServer(http.Dir(staticFilesPath)))
	http.HandleFunc("/results", resultEndpoint)
}

func StartServer(port string, resultGetter func() (interface{}, error)) error {
	resultFunc = resultGetter
	log.Println("Starting SpectroDashboard service")
	return http.ListenAndServe(":"+port, nil)
}

func resultEndpoint(w http.ResponseWriter, r *http.Request) {
	results, err := resultFunc()
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
