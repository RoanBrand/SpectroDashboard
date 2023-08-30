package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/RoanBrand/SpectroDashboard/log"
)

var server http.Server

var resultsFunc func() ([]byte, error)
var furnaceResultFunc func(furnaces []string, tSamplesOnly bool) (interface{}, error)

func SetupServer(
	staticFilesPath string,
	resultsGetter func() ([]byte, error),
	furnaceResultGetter func([]string, bool) (interface{}, error),
) {
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

	resultsFunc = resultsGetter
	furnaceResultFunc = furnaceResultGetter
}

func StartServer(port string) error {
	log.Println("Starting SpectroDashboard service")
	//return http.ListenAndServe(":"+port, nil)
	server.Addr = ":" + port
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func StopServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return server.Shutdown(ctx)
}

func resultEndpoint(w http.ResponseWriter, r *http.Request) {
	resp, err := resultsFunc()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
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
