package xml_spectro

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/http"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/shopwaredb"
	"github.com/RoanBrand/SpectroDashboard/xml_spectro/fileparser"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config
	sdb  *shopwaredb.ShopwareDB

	ctx  context.Context
	ctxD context.CancelFunc

	// result cache
	cLock    sync.RWMutex
	cExpires time.Time
	cResult  []byte
}

func NewApp() *app {
	ctx, cancel := context.WithCancel(context.Background())
	return &app{ctx: ctx, ctxD: cancel}
}

func (a *app) Start(s service.Service) error {
	go a.startup()
	return nil
}

func (a *app) startup() {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	conf, err := config.LoadConfig(filepath.Join(filepath.Dir(execPath), "config.json"))
	if err != nil {
		panic(err)
	}

	a.conf = conf

	if a.conf.ShopwareDB.Address != "" {
		a.sdb = shopwaredb.SetupShopwareDB(a.conf)
	}

	if err = a.getAndSaveNewResults(); err != nil {
		panic(err)
	}

	go a.runRoutineJob()

	log.Setup(filepath.Join(filepath.Dir(execPath), "spectrodashboard.log"), conf.DebugMode)
	http.SetupServer(
		filepath.Join(filepath.Dir(execPath), "static"),
		a.getAllResultsAPI,
		a.getLastFurnaceResultAPI,
	)

	if err = http.StartServer(conf.HTTPServerPort); err != nil {
		panic(err)
	}
}

func (a *app) Stop(s service.Service) error {
	a.ctxD()
	err1 := http.StopServer()
	err2 := a.sdb.Stop()

	if err1 != nil {
		if err2 != nil {
			return fmt.Errorf("failed to stop http server: %w and %w", err1, err2)
		}

		return fmt.Errorf("failed to stop http server: %w", err1)
	}
	if err2 != nil {
		return err2
	}

	return nil
}

func (a *app) runRoutineJob() {
	interval := time.Second * 30
	t := time.NewTimer(interval)

	for {
		select {
		case <-t.C:
			if _, err := a.getAllResultsAPI(); err != nil {
				log.Println("failed to run routine job:", err)
			}

			t.Reset(interval)

		case <-a.ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

// gets latest test sample results and saves them in the cache.
// not concurrent safe
func (a *app) getAndSaveNewResults() error {
	latestRecs, err := fileparser.GetResults(a.conf.DataSource, a.conf.NumberOfResults)
	if err != nil {
		return err
	}

	// insert shopware
	if a.sdb != nil {
		if err = a.sdb.InsertNewXMLResults(latestRecs); err != nil {
			log.Println("failed to inser new records into shopware DB:", err)
		}
	}

	// update latest insertedIntoShopWareDate

	// warm cache
	resJson, err := json.Marshal(latestRecs)
	if err != nil {
		return err
	}

	a.cResult = resJson
	a.cExpires = time.Now().Add(time.Second * 5)
	return nil
}

func (a *app) getAllResultsAPI() ([]byte, error) {
	a.cLock.RLock()
	if time.Now().Before(a.cExpires) {
		defer a.cLock.RUnlock()
		return a.cResult, nil
	}

	a.cLock.RUnlock()
	a.cLock.Lock()
	defer a.cLock.Unlock()

	if time.Now().Before(a.cExpires) {
		return a.cResult, nil
	}

	err := a.getAndSaveNewResults()
	if err != nil {
		return nil, err
	}

	return a.cResult, nil
}

func (a *app) getLastFurnaceResultAPI(furnaces []string, tSamplesOnly bool) (interface{}, error) {
	return fileparser.GetLastFurnaceResults(a.conf.DataSource, furnaces)
}
