package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/http"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/mdb_spectro"
	"github.com/RoanBrand/SpectroDashboard/sample"
	"github.com/kardianos/service"
)

type app struct{}

func (p *app) Start(s service.Service) error {
	go p.run()
	return nil
}
func (p *app) run() {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	conf, err := config.LoadConfig(filepath.Join(filepath.Dir(execPath), "config.json"))
	if err != nil {
		panic(err)
	}

	log.Setup(filepath.Join(filepath.Dir(execPath), "spectrodashboard.log"), conf.DebugMode)
	http.SetupServer(filepath.Join(filepath.Dir(execPath), "static"))

	err = http.StartServer(conf.HTTPServerPort, func() (interface{}, error) {
		results, err := getResults(conf)
		if err != nil {
			return nil, err
		}
		return results, nil
	})
	if err != nil {
		panic(err)
	}
}
func (p *app) Stop(s service.Service) error {
	return nil
}

func main() {
	svcFlag := flag.String("service", "", "Control the system service.")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "SpectroDashboard",
		DisplayName: "Spectrometer Dashboard App",
		Description: "Provides webpage that displays latest spectrometer results",
	}

	prg := &app{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if *svcFlag != "" {
		err = service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}

	logger, err := s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

var results []sample.Record

func getResults(conf *config.Config) ([]sample.Record, error) {
	results = results[:0]
	var remoteRes []sample.Record
	remoteDone := make(chan struct{})

	if conf.RemoteMachineAddress != "" {
		errOccurred := func(err ...interface{}) {
			log.Println("Error retrieving remote results from", conf.RemoteMachineAddress, ":", err)
		}
		go func() {
			defer func() { remoteDone <- struct{}{} }()

			resp, err := http.GetRemoteResults(conf.RemoteMachineAddress)
			if err != nil {
				errOccurred(err)
				return
			}
			if resp.StatusCode != 200 {
				errOccurred(resp.StatusCode, " ", resp.Status)
			}
			defer resp.Body.Close()

			dec := json.NewDecoder(resp.Body)
			err = dec.Decode(&remoteRes)
			if err != nil {
				errOccurred(err)
				return
			}
		}()

	}

	res, err := mdb_spectro.GetResults(conf.DataSource, conf.NumberOfResults, conf.ElementOrder)
	if err == nil {
		results = append(results, res...)
		if len(res) == 0 {
			log.Println("0 results found in", conf.DataSource)
		}
	} else {
		log.Println("Error retrieving local results from", conf.DataSource, ":", err)
	}

	if conf.RemoteMachineAddress != "" {
		<-remoteDone
		results = append(results, remoteRes...)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].TimeStamp.After(results[j].TimeStamp)
	})

	// limit results after merge
	if len(results) > conf.NumberOfResults {
		results = results[:conf.NumberOfResults]
	}

	return results, nil
}
