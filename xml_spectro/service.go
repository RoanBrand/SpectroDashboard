package xml_spectro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/http"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/xml_spectro/fileparser"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config
}

func NewApp() *app {
	return &app{}
}

func (a *app) Start(s service.Service) error {
	go a.run()
	return nil
}

func (a *app) run() {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	conf, err := config.LoadConfig(filepath.Join(filepath.Dir(execPath), "config.json"))
	if err != nil {
		panic(err)
	}

	a.conf = conf

	log.Setup(filepath.Join(filepath.Dir(execPath), "spectrodashboard.log"), conf.DebugMode)
	http.SetupServer(
		filepath.Join(filepath.Dir(execPath), "static"),
		a.getAllResults,
		a.getLastFurnaceResult,
	)

	if err = http.StartServer(conf.HTTPServerPort); err != nil {
		panic(err)
	}
}

func (a *app) Stop(s service.Service) error {
	return nil
}

func (a *app) getAllResults() ([]byte, error) {
	res, err := fileparser.GetResults(a.conf.DataSource, a.conf.NumberOfResults)
	if err != nil {
		return nil, err
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].TimeStamp.After(res[j].TimeStamp)
	})

	if len(res) == 0 {
		log.Println("0 results found in", a.conf.DataSource)
	}

	return json.Marshal(res)
}

func (a *app) getLastFurnaceResult(furnaces []string, tSamplesOnly bool) (interface{}, error) {
	return fileparser.GetLastFurnaceResults(a.conf.DataSource, furnaces)
}
