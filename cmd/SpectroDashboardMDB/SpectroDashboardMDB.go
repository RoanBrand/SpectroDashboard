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

	if conf.ShopwareDB.Address != "" {
		shopwaredb.SetupShopwareDB(conf)
	}

	err = http.StartServer(conf.HTTPServerPort,
		func() (interface{}, error) {
			return getResults(conf)
		},
		func(furnaces []string, tSamplesOnly bool) (interface{}, error) {
			return getLastResultFurnaces(conf, furnaces, tSamplesOnly)
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

// result cache
var lock sync.RWMutex
var age time.Time
var cacheResult []sample.Record

// never returns an error.
func getResults(conf *config.Config) ([]sample.Record, error) {
	// check if we have a recent enough result in cache
	lock.RLock()
	if time.Since(age) < time.Second*5 {
		finalRes := make([]sample.Record, len(cacheResult))
		copy(finalRes, cacheResult)
		lock.RUnlock()
		return finalRes, nil
	}

	// is old, get write lock and perform request
	lock.RUnlock()
	lock.Lock()
	defer lock.Unlock()

	// need to check if result still old, otherwise return new result
	if time.Since(age) < time.Second*5 {
		finalRes := make([]sample.Record, len(cacheResult))
		copy(finalRes, cacheResult)
		return finalRes, nil
	}

	cacheResult = cacheResult[:0]
	var remoteSpec3Res []xml_spectro.Record
	var remoteSpec3Done chan struct{}

	// get results from remote xml spectro service
	if conf.RemoteMachineAddress != "" {
		remoteSpec3Done = make(chan struct{})
		errOccurred := func(err ...interface{}) {
			log.Println("Error retrieving remote results from", conf.RemoteMachineAddress, ":", err)
		}
		go func() {
			defer func() { close(remoteSpec3Done) }()

			resp, err := http.GetRemoteResults(conf.RemoteMachineAddress)
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

	// get results from local mdb spectro
	mdbRes, err := mdb_spectro.GetResults(conf.DataSource, conf.NumberOfResults)
	if err == nil {
		dispRes := make([]sample.Record, len(mdbRes))
		for i := range mdbRes {
			dispRes[i].SampleName = mdbRes[i].SampleName
			dispRes[i].Furnace = mdbRes[i].Furnace
			dispRes[i].TimeStamp = mdbRes[i].TimeStamp
			dispRes[i].Results = make([]sample.ElementResult, len(conf.ElementOrder))
			for el, order := range conf.ElementOrder {
				if elRes, ok := mdbRes[i].Results[el]; ok {
					dispRes[i].Results[order].Element = el
					dispRes[i].Results[order].Value = elRes
				}
			}
		}

		// add mdb results to cacheval
		cacheResult = append(cacheResult, dispRes...)
		if len(dispRes) == 0 {
			log.Println("0 results found in", conf.DataSource)
		}
	} else {
		log.Println("Error retrieving local results from", conf.DataSource, ":", err)
	}

	// add xml results to cacheval
	if conf.RemoteMachineAddress != "" {
		<-remoteSpec3Done
		remSampleRes := make([]sample.Record, len(remoteSpec3Res))
		for i, r := range remoteSpec3Res {
			rec := &remSampleRes[i]
			rec.SampleName = r.ID
			rec.Furnace = r.Furnace
			rec.TimeStamp = r.TimeStamp
			rec.Results = make([]sample.ElementResult, len(conf.ElementOrder))
			for el, order := range conf.ElementOrder {
				if elRes, ok := r.Results[el]; ok {
					rec.Results[order].Element = el
					rec.Results[order].Value = elRes
				}
			}
		}
		cacheResult = append(cacheResult, remSampleRes...)
	}

	sort.Slice(cacheResult, func(i, j int) bool {
		return cacheResult[i].TimeStamp.After(cacheResult[j].TimeStamp)
	})

	// format results for foundry shopware db storage
	var recsForShopware []sample.Record
	if conf.ShopwareDB.Address != "" {
		recsForShopware = make([]sample.Record, len(mdbRes)+len(remoteSpec3Res))
		shopwareDBOrderMDB := []string{"Ni", "Mo", "Co", "Nb", "V", "W", "Mg", "Bi", "Ca", "Fe"}

		for i := range mdbRes {
			// add results from tv
			rRes := &recsForShopware[i]
			rRes.Spectro = 2
			rRes.SampleName = mdbRes[i].SampleName
			rRes.Furnace = mdbRes[i].Furnace
			rRes.TimeStamp = mdbRes[i].TimeStamp
			rRes.Results = make([]sample.ElementResult, len(conf.ElementOrder)+len(shopwareDBOrderMDB))
			for el, order := range conf.ElementOrder {
				if elRes, ok := mdbRes[i].Results[el]; ok {
					rRes.Results[order].Element = el
					rRes.Results[order].Value = elRes
				}
			}

			// add results for extra elements from MDB. tv results + extra elements
			for order, el := range shopwareDBOrderMDB {
				if elRes, ok := mdbRes[i].Results[el]; ok {
					rRes.Results[len(conf.ElementOrder)+order].Element = el
					rRes.Results[len(conf.ElementOrder)+order].Value = elRes
				}
			}
		}

		remDBOrderXML := []string{"Ni", "Mo", "Co", "Nb", "V", "W", "Mg", "Bi", "Ca", "As", "Sb", "Te", "Fe"}

		// add results from xml spectro 3
		for i, r := range remoteSpec3Res {
			rRes := &recsForShopware[len(mdbRes)+i]
			rRes.Spectro = 3
			rRes.SampleName = r.ID
			rRes.Furnace = r.Furnace
			rRes.TimeStamp = r.TimeStamp
			rRes.Results = make([]sample.ElementResult, len(conf.ElementOrder)+len(remDBOrderXML))
			for el, order := range conf.ElementOrder {
				if elRes, ok := r.Results[el]; ok {
					rRes.Results[order].Element = el
					rRes.Results[order].Value = elRes
				}
			}

			// add results for extra elements from XML
			for order, el := range remDBOrderXML {
				if elRes, ok := r.Results[el]; ok {
					rRes.Results[len(conf.ElementOrder)+order].Element = el
					rRes.Results[len(conf.ElementOrder)+order].Value = elRes
				}
			}
		}

		// sort
		sort.Slice(recsForShopware, func(i, j int) bool {
			return recsForShopware[i].TimeStamp.After(recsForShopware[j].TimeStamp)
		})
	}

	// limit results after merge
	if len(cacheResult) > conf.NumberOfResults {
		cacheResult = cacheResult[:conf.NumberOfResults]
	}

	finalRes := make([]sample.Record, len(cacheResult))
	copy(finalRes, cacheResult)
	age = time.Now()

	// go through all results, insert all into remote table that are newer than last inserted
	if conf.ShopwareDB.Address != "" {
		go func(res []sample.Record) {
			/*if conf.DebugMode {
				log.Printf("forwarding results to remote DB: %+v\n", res)
			}*/
			if err = shopwaredb.InsertNewResults(res, conf.DebugMode); err != nil {
				log.Println("Error inserting new record into remote database:", err)
			}
		}(recsForShopware)
	}

	return finalRes, nil
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
