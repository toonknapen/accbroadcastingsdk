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

func OnCarUpdate(carUpdate network.CarUpdate) {
	log.Println("Recvd RealtimeCarUpdateMsgType")
	log.Println(carUpdate)
	log.Printf("  laps:%d delta:%d", carUpdate.Laps, carUpdate.Delta)
	log.Println("  last-lap:", carUpdate.LastLap.LapTimeMs, " splits:", carUpdate.LastLap.Splits)
	log.Println("  current-lap:", carUpdate.CurrentLap.LapTimeMs, " splits:", carUpdate.CurrentLap.Splits)
}

func main() {
	var wg sync.WaitGroup

	client := network.Client{Wg: &wg, OnEntryList: OnEntryList, OnEntryListCar: OnEntryListCar, OnCarUpdate: OnCarUpdate}

	wg.Add(1)
	go client.ConnectAndRun("127.0.0.1:9000", "foobar", "asd", 5000, "")
	time.Sleep(10000 * time.Second)
	client.Disconnect()
}
