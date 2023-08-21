package main

import (
	"encoding/json"
	"flag"
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
	"github.com/RoanBrand/SpectroDashboard/xml_spectro"
	"github.com/kardianos/service"
)

type app struct {
	conf *config.Config
}

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

	p.conf = conf

	log.Setup(filepath.Join(filepath.Dir(execPath), "spectrodashboard.log"), conf.DebugMode)
	http.SetupServer(
		filepath.Join(filepath.Dir(execPath), "static"),
		p.getResults,
		func(furnaces []string, tSamplesOnly bool) (interface{}, error) {
			return getLastResultFurnaces(conf, furnaces, tSamplesOnly)
		},
	)

	if conf.ShopwareDB.Address != "" {
		shopwaredb.SetupShopwareDB(conf)
	}

	if err = http.StartServer(conf.HTTPServerPort); err != nil {
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

// result cache
var cLock sync.RWMutex
var cAge time.Time
var cacheResult []byte

// never returns an error.
func (p *app) getResults() ([]byte, error) {
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
	var remoteSpec3Res []xml_spectro.Record
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

	var allResults = mdbRes

	// add spectro 3 xml results to cacheval
	if p.conf.RemoteMachineAddress != "" {
		<-remoteSpec3Done
		for _, r := range remoteSpec3Res {
			rec := &sample.Record{}
			rec.SampleName = r.ID
			rec.Furnace = r.Furnace
			rec.TimeStamp = r.TimeStamp
			rec.Results = make([]sample.ElementResult, len(p.conf.ElementOrder))
			rec.Spectro = 3
			rec.ResultsMap = r.Results

			for el, order := range p.conf.ElementOrder {
				if elRes, ok := r.Results[el]; ok {
					rec.Results[order].Element = el
					rec.Results[order].Value = elRes
				}
			}

			allResults = append(allResults, rec)
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].TimeStamp.After(allResults[j].TimeStamp)
	})

	// limit results after merge
	if len(allResults) > p.conf.NumberOfResults {
		allResults = allResults[:p.conf.NumberOfResults]
	}

	// go through all results, insert all into remote table that are newer than last inserted
	if p.conf.ShopwareDB.Address != "" {
		go func(res []*sample.Record) {
			/*if p.conf.DebugMode {
				log.Printf("forwarding results to remote DB: %+v\n", res)
			}*/
			if err = shopwaredb.InsertNewResults(res, p.conf.DebugMode); err != nil {
				log.Println("Error inserting new record into remote database:", err)
			}
		}(allResults)
	}

	resJson, err := json.Marshal(allResults)
	if err != nil {
		return nil, err
	}

	cacheResult = resJson
	cAge = time.Now()
	return resJson, nil
}

func getLastResultFurnaces(conf *config.Config, furnaces []string, tSamplesOnly bool) (interface{}, error) {
	// get latest results from remote xml spectro 3 service
	var remoteRes []xml_spectro.Record
	var remoteDone chan struct{}
	if conf.RemoteMachineAddress != "" {
		remoteDone = make(chan struct{})
		errOccurred := func(err ...interface{}) {
			log.Println("Error retrieving remote results from", conf.RemoteMachineAddress, ":", err)
		}
		go func() {
			defer func() { close(remoteDone) }()

			resp, err := http.GetRemoteLatestFurnacesResults(conf.RemoteMachineAddress, furnaces)
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
	lastFurnaceResults, err := mdb_spectro.GetLastFurnaceResults(conf.DataSource, furnaces, tSamplesOnly)
	if err != nil {
		return nil, err
	}

	// spectro 3
	if conf.RemoteMachineAddress != "" {
		<-remoteDone
		for i, lfr := range lastFurnaceResults {
			for _, remlfr := range remoteRes {
				if remlfr.Furnace != lfr.Furnace {
					continue
				}

				if remlfr.TimeStamp.Before(lfr.TimeStamp) {
					continue
				}

				lastFurnaceResults[i].SampleName = remlfr.ID
				lastFurnaceResults[i].TimeStamp = remlfr.TimeStamp
				break
			}
		}
	}

	return lastFurnaceResults, nil
}
