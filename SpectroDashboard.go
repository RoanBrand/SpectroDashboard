package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/kardianos/service"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger service.Logger
var conf *config

type app struct{}

func (p *app) Start(s service.Service) error {
	go p.run()
	return nil
}
func (p *app) run() {
	var err error
	conf, err = loadConfig("config.json")
	if err != nil {
		panic(err)
	}

	log.SetOutput(&lumberjack.Logger{
		Filename:   filepath.Join(filepath.Dir(execPath), "spectrodashboard.log"),
		MaxBackups: 3,
		MaxAge:     28, //days
	})

	setupHTTPServer()
	err = startHTTPServer(conf.HTTPServerPort)
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

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
