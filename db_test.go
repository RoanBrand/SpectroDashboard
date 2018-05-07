package main

import (
	"testing"
	"runtime"
)

const dsn = "Provider=Microsoft.ACE.OLEDB.12.0;Data Source=A:/Projects/SpectroDisplay/AccessDB Example/SpvDB_MeasureResults.mdb;"
const numResults = 100
const numRetrievals = 1000

func logStats(t *testing.T) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("\nAlloc = %v\nTotalAlloc = %v\nSys = %v\nNumGC = %v\n\n", m.Alloc / 1024, m.TotalAlloc / 1024, m.Sys / 1024, m.NumGC)
}

func TestResultsRetrieval(t *testing.T) {
	logStats(t)

	err := startDBConn(dsn)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < numRetrievals; i++ {
		_, err := queryResults(numResults)
		if err != nil {
			logStats(t)
			t.Fatalf("Error retrieving results on iteration %d: %s", i, err)
		}
	}
	logStats(t)
}

func BenchmarkResultRetrieval(b *testing.B) {
	err := startDBConn(dsn)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := queryResults(numResults)
		if err != nil {
			b.Fatalf("Error retrieving results on iteration %d: %s", i, err)
		}
	}
}
