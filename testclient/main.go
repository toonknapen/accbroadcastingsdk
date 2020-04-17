package main

import (
	"github.com/toonknapen/accbroadcastingsdk/network"
	"log"
	"sync"
	"time"
)

func OnEntryList(entryList network.EntryList) {
	log.Println("EntryList:", entryList)
}

func OnEntryListCar(entryListCar network.EntryListCar) {
	log.Println("EntryListCar:", entryListCar)
}

func OnRealTimeUpdate(realTimeUpdate network.RealTimeUpdate) {
	log.Println("Recvd RealTimeUpdate:")
	log.Println("  SessionType:", realTimeUpdate.SessionType)
	log.Println("  Phase:", realTimeUpdate.Phase)
	log.Println("  SessionTime:", realTimeUpdate.SessionTime)
	log.Println("  SessionEndTime:", realTimeUpdate.SessionEndTime)
	log.Println("  FocusedCarIndex:", realTimeUpdate.FocusedCarIndex)
	log.Println("  ActiveCameraSet:", realTimeUpdate.ActiveCameraSet)
	log.Println("  IsReplayPlaying:", realTimeUpdate.IsReplayPlaying)
	log.Println("  TimeOfDay:", realTimeUpdate.TimeOfDay)
}

func OnRealTimeCarUpdate(realTimeCarUpdate network.RealTimeCarUpdate) {
	log.Println("Recvd RealtimeCarUpdateMsgType")
	log.Println(realTimeCarUpdate)
	log.Printf("  driverId:%d driverCount:%d", realTimeCarUpdate.DriverId, realTimeCarUpdate.DriverCount)
	log.Printf("  posX:%f, posY:%f", realTimeCarUpdate.WorldPosX, realTimeCarUpdate.WorldPosY)
	log.Printf("  carLocation:%d", realTimeCarUpdate.CarLocation)
	log.Printf("  laps:%d delta:%d", realTimeCarUpdate.Laps, realTimeCarUpdate.Delta)
	log.Println("  last-lap:", realTimeCarUpdate.LastLap.LapTimeMs, " splits:", realTimeCarUpdate.LastLap.Splits)
	log.Println("  current-lap:", realTimeCarUpdate.CurrentLap.LapTimeMs, " splits:", realTimeCarUpdate.CurrentLap.Splits)
}

func OnBroadCastEvent(broadCastEvent network.BroadCastEvent) {
	log.Println("Recvd BroadCastEvent:", broadCastEvent)
}

func main() {
	var wg sync.WaitGroup

	accClient := network.Client{
		Wg:                  &wg,
		OnEntryList:         OnEntryList,
		OnEntryListCar:      OnEntryListCar,
		OnRealTimeUpdate:    OnRealTimeUpdate,
		OnRealTimeCarUpdate: OnRealTimeCarUpdate,
		OnBroadCastEvent:    OnBroadCastEvent,
	}

	wg.Add(1)
	go accClient.ConnectAndRun("127.0.0.1:9000", "foobar", "asd", 5000, "")

	time.Sleep(10000 * time.Second)
	accClient.Disconnect()
}
