package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var execPath string

type config struct {
	HTTPServerPort        string   `json:"http_server_port"`
	ElementsToDisplay     []string `json:"elements_to_display"`
	NumberOfResults       int      `json:"number_of_results"`       // number of latest results returned to client
	ClientRefreshInterval int      `json:"client_refresh_interval"` // period in (s) between when clients reload results

	Machines []Machine `json:"machines"`

	elementOrder map[string]int
}

type Machine struct {
	Name        string `json:"name"`
	DataType    string `json:"data_type"`    // either 'xml' or 'mdb'
	SampleLimit int    `json:"sample_limit"` // limit number of samples requested
	DataSource  string `json:"data_source"`  // If xml: folder of xml files. If mdb: path to mdb file database.
}

func loadConfig(filename string) (*config, error) {
	var err error
	execPath, err = os.Executable()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(filepath.Dir(execPath), filename))
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(f)
	conf := &config{
		HTTPServerPort:        "80",
		ElementsToDisplay:     []string{"C", "Si", "Mn", "P", "S", "Cu", "Cr", "Al", "Ti", "Sn", "Zn", "Pb"},
		NumberOfResults:       20,
		ClientRefreshInterval: 10,
	}

	err = dec.Decode(conf)
	if err != nil {
		return nil, err
	}

	conf.elementOrder = make(map[string]int, len(conf.ElementsToDisplay))
	for i, el := range conf.ElementsToDisplay {
		conf.elementOrder[el] = i
	}

	// validation
	/*if conf.DataSource == "" {
		return nil, errors.New("no data_source provided in config file")
	}*/

	return conf, nil
}
