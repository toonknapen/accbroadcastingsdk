package main

import (
	"encoding/json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/toonknapen/accbroadcastingsdk/v2/network"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func OnRealTimeUpdate(realTimeUpdate network.RealTimeUpdate) {
	raw, err := json.Marshal(realTimeUpdate)
	if err != nil {
		log.Error().Msgf("Error while marshaling realtimeupdate: %v", err)
		return
	}
	log.Info().Msgf("RealTimeUpdate: %s", raw)
}

func OnRealTimeCarUpdate(realTimeCarUpdate network.RealTimeCarUpdate) {
	raw, err := json.Marshal(realTimeCarUpdate)
	if err != nil {
		log.Error().Msgf("Error while marshaling realtimecarupdate: %v", err)
		return
	}
	log.Debug().Msgf("RealtimeCarUpdate: %s", raw)
}

func OnEntryList(entryList network.EntryList) {
	raw, err := json.Marshal(entryList)
	if err != nil {
		log.Error().Msgf("Error while marshaling entrylist: %v", err)
		return
	}
	log.Debug().Msgf("EntryList: %s", raw)
}

func OnEntryListCar(entryListCar network.EntryListCar) {
	raw, err := json.Marshal(entryListCar)
	if err != nil {
		log.Error().Msgf("Error while marshaling entrylistcar: %v", err)
		return
	}
	log.Debug().Msgf("EntryListCar: %s", raw)
}

func OnTrackData(trackData network.TrackData) {
	raw, err := json.Marshal(trackData)
	if err != nil {
		log.Error().Msgf("Error while marshaling trackdata: %v", err)
		return
	}
	log.Debug().Msgf("TrackData: %s", raw)
}

func OnBroadCastEvent(broadCastEvent network.BroadCastEvent) {
	log.Debug().Msgf("BroadCastEvent: %v", broadCastEvent)
}

func SetupCloseHandler(client *network.Client) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msg("\r- Ctrl+C pressed in Terminal")
		client.Disconnect()
		os.Exit(0)
	}()
}
func main() {
	network.Logger = network.Logger.Output(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true, TimeFormat: zerolog.TimeFieldFormat})
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true, TimeFormat: zerolog.TimeFieldFormat})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	accClient := network.Client{
		OnRealTimeUpdate:    OnRealTimeUpdate,
		OnRealTimeCarUpdate: OnRealTimeCarUpdate,
		OnEntryList:         OnEntryList,
		OnEntryListCar:      OnEntryListCar,
		OnTrackData:         OnTrackData,
		OnBroadCastEvent:    OnBroadCastEvent,
	}

	SetupCloseHandler(&accClient)
	go accClient.ConnectAndRun("127.0.0.1:9000", "pitwall", "asd", 1000, "", 5000)

	time.Sleep(24 * time.Hour)
	accClient.Disconnect()
}
