package SpectroDashboard_test

import (
	"encoding/json"
	"fmt"
	"github.com/RoanBrand/SpectroDashboard/config"
	ht "github.com/RoanBrand/SpectroDashboard/http"
	"github.com/RoanBrand/SpectroDashboard/log"
	"github.com/RoanBrand/SpectroDashboard/mdb_spectro"
	"github.com/RoanBrand/SpectroDashboard/sample"
	"math/rand"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

const (
	homePath      = "A:/Projects/SpectroDisplay/SpectroDashboard"
	numResults    = 20
	numRetrievals = 500
)

func TestResultsRetrieval(t *testing.T) {
	done := make(chan struct{})
	errPipe := make(chan string)

	conf, err := config.LoadConfig(filepath.Join(homePath, "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	ht.SetupServer(filepath.Join(homePath, "static"))

	go func() {
		if err := ht.StartServer(conf.HTTPServerPort, func() (interface{}, error) {
			results, err := getResults(conf)
			if err != nil {
				return nil, err
			}
			return results, nil
		}); err != nil {
			errPipe <- err.Error()
		}
	}()

	time.Sleep(time.Millisecond * 10)
	var wg sync.WaitGroup
	wg.Add(numRetrievals)

	for i := 0; i < numRetrievals; i++ {
		go func(i int) {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

			c := http.Client{}
			resp, err := c.Get("http://localhost:80/results")
			if err != nil {
				fmt.Println(i, "error-", err)
				errPipe <- fmt.Sprintf("Error retrieving results on iteration %d: %s", i, err)
				fmt.Println(i, "error2-", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errPipe <- fmt.Sprintf("Error retrieving results on iteration %d: %s", i, err)
				return
			}

			var res []sample.Record
			dec := json.NewDecoder(resp.Body)
			err = dec.Decode(&res)
			if resp.StatusCode != http.StatusOK {
				errPipe <- fmt.Sprintf("Error decoding results on iteration %d: %s", i, err)
				return
			}

			if len(res) != numResults {
				errPipe <- fmt.Sprintf("result length %d, should be %d", len(res), numResults)
			}
		}(i)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case errMsg := <-errPipe:
		t.Fatal(errMsg)
	case <-done:
	}
}

// result cache
var lock sync.RWMutex
var age time.Time
var cacheResult []sample.Record

func getResults(conf *config.Config) ([]sample.Record, error) {
	// check if we have a recent enough result in cache
	lock.RLock()
	if time.Now().Sub(age) < time.Second*5 {
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
	if time.Now().Sub(age) < time.Second*5 {
		finalRes := make([]sample.Record, len(cacheResult))
		copy(finalRes, cacheResult)
		return finalRes, nil
	}

	cacheResult = cacheResult[:0]
	var remoteRes []sample.Record
	remoteDone := make(chan struct{})

	if conf.RemoteMachineAddress != "" {
		errOccurred := func(err ...interface{}) {
			log.Println("Error retrieving remote results from", conf.RemoteMachineAddress, ":", err)
		}
		go func() {
			defer func() { remoteDone <- struct{}{} }()

			resp, err := ht.GetRemoteResults(conf.RemoteMachineAddress)
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
		cacheResult = append(cacheResult, res...)
		if len(res) == 0 {
			log.Println("0 results found in", conf.DataSource)
		}
	} else {
		log.Println("Error retrieving local results from", conf.DataSource, ":", err)
	}

	if conf.RemoteMachineAddress != "" {
		<-remoteDone
		cacheResult = append(cacheResult, remoteRes...)
	}

	sort.Slice(cacheResult, func(i, j int) bool {
		return cacheResult[i].TimeStamp.After(cacheResult[j].TimeStamp)
	})

	// limit results after merge
	if len(cacheResult) > conf.NumberOfResults {
		cacheResult = cacheResult[:conf.NumberOfResults]
	}

	finalRes := make([]sample.Record, len(cacheResult))
	copy(finalRes, cacheResult)
	age = time.Now()

	return finalRes, nil
}
