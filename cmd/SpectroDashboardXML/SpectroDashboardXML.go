package main

import (
	"flag"
	"os"
	"path/filepath"
	"sort"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/http"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/sample"
	"github.com/RoanBrand/SpectroDashboard/xml_spectro"
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
	}, nil)
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
		Name:        "SpectroDashboardXML",
		DisplayName: "Spectrometer Dashboard App",
		Description: "Provides API for latest XML spectrometer results",
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

func getResults(conf *config.Config) ([]sample.Record, error) {
	res, err := xml_spectro.GetResults(conf.DataSource, conf.NumberOfResults, conf.ElementOrder)
	if err != nil {
		return nil, err
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].TimeStamp.After(res[j].TimeStamp)
	})

	if len(res) == 0 {
		log.Println("0 results found in", conf.DataSource)
	}

	return res, nil
}
