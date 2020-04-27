package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/toonknapen/accbroadcastingsdk/network"
	"os"
	"sync"
	"time"
)

func OnEntryList(entryList network.EntryList) {
	log.Debug().Msgf("EntryList: %v", entryList)
}

func OnEntryListCar(entryListCar network.EntryListCar) {
	log.Debug().Msgf("EntryListCar: %v", entryListCar)
}

func OnTrackData(trackData network.TrackData) {
	log.Debug().Msgf("TrackData: %v", trackData)
}

func OnRealTimeUpdate(realTimeUpdate network.RealTimeUpdate) {
	log.Debug().Msgf("RealTimeUpdate: %v", realTimeUpdate)
}

func OnRealTimeCarUpdate(realTimeCarUpdate network.RealTimeCarUpdate) {
	log.Debug().Msgf("RealtimeCarUpdate: %v", realTimeCarUpdate)
}

func OnBroadCastEvent(broadCastEvent network.BroadCastEvent) {
	log.Debug().Msgf("BroadCastEvent: %v", broadCastEvent)
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	var wg sync.WaitGroup

	accClient := network.Client{
		Wg:                  &wg,
		OnEntryList:         OnEntryList,
		OnEntryListCar:      OnEntryListCar,
		OnTrackData:         OnTrackData,
		OnRealTimeUpdate:    OnRealTimeUpdate,
		OnRealTimeCarUpdate: OnRealTimeCarUpdate,
		OnBroadCastEvent:    OnBroadCastEvent,
	}

	wg.Add(1)
	go accClient.ConnectAndRun("127.0.0.1:9000", "foobar", "asd", 1000, "", 5000)

	time.Sleep(10000 * time.Second)
	accClient.Disconnect()
}
