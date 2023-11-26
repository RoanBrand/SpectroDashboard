package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/RoanBrand/SpectroDashboard/config"
	"github.com/RoanBrand/SpectroDashboard/http"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/mdb_spectro"
	"github.com/RoanBrand/SpectroDashboard/sample"
	"github.com/RoanBrand/SpectroDashboard/shopwaredb"
	"github.com/RoanBrand/SpectroDashboard/xml_spectro/fileparser"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config
	sdb  *shopwaredb.ShopwareDB

	ctx  context.Context
	ctxD context.CancelFunc
}

func (p *app) Start(s service.Service) error {
	go p.startup()
	return nil
}
func (p *app) startup() {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	conf, err := config.LoadConfig(filepath.Join(filepath.Dir(execPath), "config.json"))
	if err != nil {
		panic(err)
	}

	p.conf = conf

	if conf.ShopwareDB.Address != "" {
		p.sdb = shopwaredb.SetupShopwareDB(conf)
	}

	go p.runRoutineJob()

	log.Setup(filepath.Join(filepath.Dir(execPath), "spectrodashboard.log"), conf.DebugMode)
	http.SetupServer(
		filepath.Join(filepath.Dir(execPath), "static"),
		p.getResultsAPI,
		p.getLastResultFurnacesAPI,
	)

	if err = http.StartServer(conf.HTTPServerPort); err != nil {
		panic(err)
	}
}
func (p *app) Stop(s service.Service) error {
	p.ctxD()
	err1 := http.StopServer()
	err2 := p.sdb.Stop()

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
			if _, err := a.getResultsAPI(); err != nil {
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

func main() {
	svcFlag := flag.String("service", "", "Control the system service.")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "SpectroDashboard",
		DisplayName: "Spectrometer Dashboard App",
		Description: "Provides webpage that displays latest spectrometer results",
	}

	ctx, cancel := context.WithCancel(context.Background())
	prg := &app{ctx: ctx, ctxD: cancel}
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

// result cache
var cLock sync.RWMutex
var cAge time.Time
var cacheResult []byte

// never returns an error.
func (p *app) getResultsAPI() ([]byte, error) {
	// check if cache recent enough
	cLock.RLock()
	if time.Since(cAge) < time.Second*5 {
		defer cLock.RUnlock()
		return cacheResult, nil
	}

	// is old, get write lock and perform request
	cLock.RUnlock()
	cLock.Lock()
	defer cLock.Unlock()

	// need to check if result still old, otherwise return new result
	if time.Since(cAge) < time.Second*5 {
		return cacheResult, nil
	}

	// get new results and update cache
	var remoteSpec3Res []fileparser.Record
	var remoteSpec3Done chan struct{}

	// get results from xml spectro 3 service
	if p.conf.RemoteMachineAddress != "" {
		remoteSpec3Done = make(chan struct{})
		errOccurred := func(err ...interface{}) {
			log.Println("Error retrieving remote results from", p.conf.RemoteMachineAddress, ":", err)
		}
		go func() {
			defer func() { close(remoteSpec3Done) }()

			resp, err := http.GetRemoteResults(p.conf.RemoteMachineAddress)
			if err != nil {
				errOccurred(err)
				return
			}
			if resp.StatusCode != 200 {
				errOccurred(resp.StatusCode, " ", resp.Status)
			}
			defer resp.Body.Close()

			err = json.NewDecoder(resp.Body).Decode(&remoteSpec3Res)
			if err != nil {
				errOccurred(err)
				return
			}
		}()
	}

	// get results from local mdb spectro 2
	mdbRes, err := mdb_spectro.GetResults(p.conf.DataSource, p.conf.NumberOfResults)
	if err != nil {
		log.Println("Error retrieving local results from", p.conf.DataSource, ":", err)
	} else {
		// lookup and prepare elements to display
		for _, r := range mdbRes {
			r.Results = make([]sample.ElementResult, len(p.conf.ElementOrder))
			r.Spectro = 2

			for el, order := range p.conf.ElementOrder {
				if elRes, ok := r.ResultsMap[el]; ok {
					r.Results[order].Element = el
					r.Results[order].Value = elRes
				}
			}
		}

		if len(mdbRes) == 0 {
			log.Println("0 results found in", p.conf.DataSource)
		}
	}

	// go through all results, insert all into remote table that are newer than last inserted
	if p.sdb != nil {
		if err = p.sdb.InsertNewMDBResults(mdbRes); err != nil {
			log.Println("Error inserting new record into remote database:", err)
		}
	}

	var allResults = mdbRes

	// add spectro 3 xml results to cacheval
	if p.conf.RemoteMachineAddress != "" {
		<-remoteSpec3Done
		for i := range remoteSpec3Res {
			xmlR := &remoteSpec3Res[i]
			sR := &sample.Record{
				SampleName: xmlR.ID,
				Furnace:    xmlR.Furnace,
				TimeStamp:  xmlR.TimeStamp,
				Results:    make([]sample.ElementResult, len(p.conf.ElementOrder)),
				Spectro:    3,
				ResultsMap: xmlR.Results,
			}

			for el, order := range p.conf.ElementOrder {
				if elRes, ok := xmlR.Results[el]; ok {
					sR.Results[order].Element = el
					sR.Results[order].Value = elRes
				}
			}

			allResults = append(allResults, sR)
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].TimeStamp.After(allResults[j].TimeStamp)
	})

	// limit results after merge for tv api
	if len(allResults) > p.conf.NumberOfResults {
		allResults = allResults[:p.conf.NumberOfResults]
	}

	resJson, err := json.Marshal(allResults)
	if err != nil {
		return nil, err
	}

	cacheResult = resJson
	cAge = time.Now()
	return resJson, nil
}

func (p *app) getLastResultFurnacesAPI(furnaces []string, tSamplesOnly bool) (interface{}, error) {
	// get latest results from remote xml spectro 3 service
	var remoteRes []fileparser.Record
	var remoteDone chan struct{}
	if p.conf.RemoteMachineAddress != "" {
		remoteDone = make(chan struct{})
		errOccurred := func(err ...interface{}) {
			log.Println("Error retrieving remote results from", p.conf.RemoteMachineAddress, ":", err)
		}
		go func() {
			defer func() { close(remoteDone) }()

			resp, err := http.GetRemoteLatestFurnacesResults(p.conf.RemoteMachineAddress, furnaces)
			if err != nil {
				errOccurred(err)
				return
			}
			if resp.StatusCode != 200 {
				errOccurred(resp.StatusCode, " ", resp.Status)
			}
			defer resp.Body.Close()

			err = json.NewDecoder(resp.Body).Decode(&remoteRes)
			if err != nil {
				errOccurred(err)
				return
			}
		}()
	}

	// spectro 2
	lastFurnaceResults, err := mdb_spectro.GetLastFurnaceResults(p.conf.DataSource, furnaces, tSamplesOnly)
	if err != nil {
		return nil, err
	}

	// spectro 3
	if p.conf.RemoteMachineAddress != "" {
		<-remoteDone
		for i := range lastFurnaceResults {
			lfr := &lastFurnaceResults[i]
			for j := range remoteRes {
				remlfr := &remoteRes[j]

				if remlfr.Furnace != lfr.Furnace {
					continue
				}

				if remlfr.TimeStamp.Before(lfr.TimeStamp) {
					continue
				}

				lfr.SampleName = remlfr.ID
				lfr.TimeStamp = remlfr.TimeStamp
				break
			}
		}
	}

	return lastFurnaceResults, nil
}
