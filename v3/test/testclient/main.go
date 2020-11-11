package main

import (
	"encoding/json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/toonknapen/accbroadcastingsdk/v3/network"
	"time"
)

var connected chan bool

func OnConnected(connectionId int32) {
	connected <- true
}

func OnDisconnected() {
	connected <- false
}

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

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	subLogger := log.With().Str("component", "ACCBroacastingSDK").Logger()

	connected = make(chan bool)

	accClient := network.Client{
		Logger:              subLogger,
		OnConnected:         OnConnected,
		OnDisconnected:      OnDisconnected,
		OnRealTimeUpdate:    OnRealTimeUpdate,
		OnRealTimeCarUpdate: OnRealTimeCarUpdate,
		OnEntryList:         OnEntryList,
		OnEntryListCar:      OnEntryListCar,
		OnTrackData:         OnTrackData,
		OnBroadCastEvent:    OnBroadCastEvent,
	}

	for i := 0; i < 10; i++ {
		go accClient.ConnectListenAndCallback("127.0.0.1:9000", "pitwall", "asd", 1000, "", 5000)
		<-connected // wait until OnConnected flags in the 'connected' channel that the connection is established.
		log.Info().Msg("Receiving messages")
		time.Sleep(10 * time.Second)
		log.Info().Msg("Disconnecting")
		accClient.RequestDisconnect()
	}
}
