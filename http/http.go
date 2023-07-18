package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroDashboard/log"
)

var resultFunc func() (interface{}, error)
var furnaceResultFunc func(furnaces []string, tSamplesOnly bool) (interface{}, error)

func SetupServer(staticFilesPath string) {
	http.Handle("/", http.FileServer(http.Dir(staticFilesPath)))
	http.HandleFunc("/results", resultEndpoint)
	http.HandleFunc("/lastfurnaceresults", lastFurnaceResult)
	http.HandleFunc("/gettime", func(w http.ResponseWriter, r *http.Request) {
		sysTime := struct {
			T time.Time `json:"t"`
		}{T: time.Now()}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&sysTime); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
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

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(results)
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

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// for tv
func GetRemoteResults(remoteAddress string) (*http.Response, error) {
	if !strings.HasPrefix(remoteAddress, "http://") {
		remoteAddress = "http://" + remoteAddress
	}
	if !strings.HasSuffix(remoteAddress, "/results") {
		remoteAddress = remoteAddress + "/results"
	}
	return http.Get(remoteAddress)
}

func GetRemoteLatestFurnacesResults(remoteAddress string, furnaces []string) (*http.Response, error) {
	if !strings.HasPrefix(remoteAddress, "http://") {
		remoteAddress = "http://" + remoteAddress
	}
	if !strings.HasSuffix(remoteAddress, "/lastfurnaceresults") {
		remoteAddress = remoteAddress + "/lastfurnaceresults"
	}
	for i, furnace := range furnaces {
		if i == 0 {
			remoteAddress += "?f="
		} else {
			remoteAddress += "&f="
		}
		remoteAddress += furnace
	}

	return http.Get(remoteAddress)
}
