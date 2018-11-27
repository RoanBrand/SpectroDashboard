package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/RoanBrand/SpectroDashboard/log"
)

var resultFunc func() (interface{}, error)
var furnaceResultFunc func(furnaces []string, tSamplesOnly bool) (interface{}, error)

func SetupServer(staticFilesPath string) {
	http.Handle("/", http.FileServer(http.Dir(staticFilesPath)))
	http.HandleFunc("/results", resultEndpoint)
	http.HandleFunc("/lastfurnaceresults", lastFurnaceResult)
}

func StartServer(port string, resultGetter func() (interface{}, error), furnaceResultGetter func([]string, bool) (interface{}, error)) error {
	resultFunc = resultGetter
	furnaceResultFunc = furnaceResultGetter
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

func lastFurnaceResult(w http.ResponseWriter, r *http.Request) {
	if furnaceResultFunc == nil {
		return
	}

	q := r.URL.Query()
	results, err := furnaceResultFunc(q["f"], q["t"] != nil && q["t"][0] == "true")
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

func GetRemoteResults(remoteAddress string) (*http.Response, error) {
	if !strings.HasPrefix(remoteAddress, "http://") {
		remoteAddress = "http://" + remoteAddress
	}
	if !strings.HasSuffix(remoteAddress, "/results") {
		remoteAddress = remoteAddress + "/results"
	}
	return http.Get(remoteAddress)
}
