package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/toonknapen/accbroadcastingsdk/v3/network"
	"os"
	"time"
)

var connectedStream chan int32
var sessionTimeStream chan float32
var disconnectedStream chan int32

func OnConnected(connectionId int32) {
	log.Info().Msgf("OnConnected: id=%d", connectionId)
	connectedStream <- connectionId
}

func OnRealTimeUpdate(realTimeUpdate network.RealTimeUpdate) {
	log.Info().Msgf("RealTimeUpdate %f", realTimeUpdate.SessionTime)
	sessionTimeStream <- realTimeUpdate.SessionTime
}

func OnDisconnected() {
	log.Info().Msg("OnDisconnected")
	disconnectedStream <- 0
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, NoColor: true, TimeFormat: zerolog.TimeFieldFormat})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	connectedStream = make(chan int32)
	sessionTimeStream = make(chan float32, 100)
	disconnectedStream = make(chan int32)

	accClient := network.Client{
		OnConnected:      OnConnected,
		OnDisconnected:   OnDisconnected,
		OnRealTimeUpdate: OnRealTimeUpdate,
	}

	// network.SetupCloseHandler(&accClient)

	for i := 0; i < 30; i++ {
		log.Info().Msgf("main loop going to connect")
		go accClient.ConnectListenAndCallback("127.0.0.1:9000", "pitwall", "asd", 250, "", 5000)

		connectionId := <-connectedStream
		log.Info().Msgf("main loop Connected: %d", connectionId)

		for i := 0; i < 5; i++ {
			<-sessionTimeStream
		}

		log.Info().Msgf("main loop requesting to disconnect")
		accClient.RequestDisconnect()
		<-disconnectedStream
		log.Info().Msgf("main loop DisConnected")

		log.Info().Msgf("length sessionTimeStream %d", len(sessionTimeStream))
		for len(sessionTimeStream) > 0 {
			<-sessionTimeStream
		}

		waitSeconds := 1
		log.Info().Msgf("waiting for %d seconds", waitSeconds)
		time.Sleep(time.Duration(waitSeconds) * time.Second)
	}
}
