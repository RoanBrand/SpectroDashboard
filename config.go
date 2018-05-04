package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var execPath string

type config struct {
	DataSource            string `json:"data_source"`
	NumberOfResults       int    `json:"number_of_results"`       // number of latest results returned to client
	ClientRefreshInterval int    `json:"client_refresh_interval"` // period in (s) between when clients reload results
	HTTPServerPort        string `json:"http_server_port"`
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
		NumberOfResults:       20,
		ClientRefreshInterval: 10,
		HTTPServerPort:        "80",
	}

	err = dec.Decode(conf)
	if err != nil {
		return nil, err
	}

	// validation
	if conf.DataSource == "" {
		return nil, errors.New("no data_source provided in config file")
	}

	return conf, nil
}
