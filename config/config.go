package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	HTTPServerPort        string   `json:"http_server_port"`
	ElementsToDisplay     []string `json:"elements_to_display"`
	NumberOfResults       int      `json:"number_of_results"`       // number of latest results returned to client
	ClientRefreshInterval int      `json:"client_refresh_interval"` // period in (s) between when clients reload results

	DataSource           string `json:"data_source"`            // If xml: folder of xml files. If mdb: path to mdb file database.
	DebugMode            bool   `json:"debug_mode"`             // print logs out to console instead of file when true
	RemoteMachineAddress string `json:"remote_machine_address"` // optional: mix results with remote spectro

	RemoteDatabase struct {
		Address    string `json:"address"`
		User       string `json:"user"`
		Password   string `json:"password"`
		Database   string `json:"database"`
		Table      string `json:"table"`
		//Elements   []string `json:"elements"`
	} `json:"remote_database"`

	ElementOrder map[string]int // internal use and just for displays
}

func LoadConfig(filePath string) (*Config, error) {
	conf := Config{
		HTTPServerPort:        "80",
		ElementsToDisplay:     []string{"C", "Si", "Mn", "P", "S", "Cu", "Cr", "Al", "Ti", "Sn", "Zn", "Pb"},
		NumberOfResults:       20,
		ClientRefreshInterval: 10,
	}
	//conf.RemoteDatabase.Elements = []string{"C", "Si", "Mn", "P", "S", "Cu", "Cr", "Al", "Ti", "Sn", "Zn", "Pb"}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(f)
	err = dec.Decode(&conf)
	if err != nil {
		return nil, err
	}

	conf.ElementOrder = make(map[string]int, len(conf.ElementsToDisplay))
	for i, el := range conf.ElementsToDisplay {
		conf.ElementOrder[el] = i
	}

	// validation
	/*if conf.DataSource == "" {
		return nil, errors.New("no data_source provided in config file")
	}*/

	return &conf, nil
}
