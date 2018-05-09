package main

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

const dsn = "Provider=Microsoft.ACE.OLEDB.12.0;Data Source=A:/Projects/SpectroDisplay/AccessDB Example/SpvDB_MeasureResults.mdb;"
const numResults = 100
const numRetrievals = 10

func logStats(t *testing.T) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("\nAlloc = %v\nTotalAlloc = %v\nSys = %v\nNumGC = %v\n\n", m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC)
}

func TestResultsRetrieval(t *testing.T) {
	logStats(t)

	/*err := startDBConn(dsn)
	if err != nil {
		t.Fatal(err)
	}*/

	errPipe := make(chan string)
	go func() {
		t.Fatal(<-errPipe)
	}()
	var wg sync.WaitGroup

	wg.Add(numRetrievals)
	for i := 0; i < numRetrievals; i++ {
		go func(i int) {
			res, err := queryResults(dsn, numResults)
			if err != nil {
				errPipe <- fmt.Sprintf("Error retrieving results on iteration %d: %s", i, err)
			} else if len(res) != 100 {
				errPipe <- fmt.Sprintf("result length %d, should be %d", len(res), numResults)
			}
			wg.Done()
		}(i)
	}
	logStats(t)
	wg.Wait()
	logStats(t)
}

func BenchmarkResultRetrieval(b *testing.B) {
	/*err := startDBConn(dsn)
	if err != nil {
		b.Fatal(err)
	}*/

	errPipe := make(chan string)
	go func() {
		b.Fatal(<-errPipe)
	}()
	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go func(i int) {
			res, err := queryResults(dsn, numResults)
			if err != nil {
				errPipe <- fmt.Sprintf("Error retrieving results on iteration %d: %s", i, err)
			} else if len(res) != 100 {
				errPipe <- fmt.Sprintf("result length %d, should be %d", len(res), numResults)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}
