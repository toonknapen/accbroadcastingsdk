package network

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"net"
	"sync"
	"time"
)

const BROADCASTING_PROTOCOL_VERSION byte = 3
const ReadBufferSize = 32 * 1024

type Client struct {
	Wg                  *sync.WaitGroup
	conn                *net.UDPConn
	OnEntryList         func(EntryList)
	OnEntryListCar      func(EntryListCar)
	OnTrackData         func(TrackData)
	OnRealTimeUpdate    func(RealTimeUpdate)
	OnRealTimeCarUpdate func(RealTimeCarUpdate)
	OnBroadCastEvent    func(BroadCastEvent)
}

func (client *Client) ConnectAndRun(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string) {
	attempt := 0

StartConnectionLoop:
	for true {
		if attempt > 0 {
			log.Info().Msg("Sleeping before retrying ...")
			time.Sleep(5 * time.Second)
		}
		attempt++

		log.Print("Connecting to:", address)

		raddr, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			log.Error().Msgf("resolving address:%v", err)
			continue StartConnectionLoop
		}

		client.conn, err = net.DialUDP("udp", nil, raddr)
		if err != nil {
			log.Error().Msgf("Error when establishing UDP connection: %v -> retrying", err)
		}

		var writeBuffer bytes.Buffer
		MarshalConnectinReq(&writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
		client.conn.SetDeadline(time.Now().Add(time.Second))
		n, err := client.conn.Write(writeBuffer.Bytes())
		if n < writeBuffer.Len() {
			log.Error().Msgf("Error causing only to write partial message -> restarting connection")
			continue StartConnectionLoop
		}
		if err != nil {
			log.Error().Msgf("Error when writing message-type: %v -> restarting connection", err)
			continue StartConnectionLoop
		}

		var readArray [ReadBufferSize]byte
		done := false
		timeoutDuration := time.Duration(2*msRealtimeUpdateInterval) * time.Millisecond
		for !done {
			// read socket
			client.conn.SetDeadline(time.Now().Add(timeoutDuration))
			n, err = client.conn.Read(readArray[:])
			if err != nil {
				log.Error().Msgf("Error when reading message-type: %v -> restarting connection", err)
				continue StartConnectionLoop
			}
			if n == ReadBufferSize {
				log.Panic().Msg("Buffer not big enough !!!")
			}

			// extract msgType
			readBuffer := bytes.NewBuffer(readArray[:n])
			msgType, err := readBuffer.ReadByte()
			if err != nil {
				log.Error().Msg("No msgType -> restarting connection")
				continue StartConnectionLoop
			}

			// data
			// var sessionCarIds []uint16

			// handle msg
			switch msgType {
			case RegistrationResultMsgType:
				log.Info().Msg("Recvd Registration")
				connectionId, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				log.Info().Msgf("Connection: %d - %d - %s", connectionId, isReadOnly, errMsg)

				writeBuffer.Reset()
				MarshalEntryListReq(&writeBuffer, connectionId)
				client.conn.Write(writeBuffer.Bytes())

				writeBuffer.Reset()
				MarshalTrackDataReq(&writeBuffer, connectionId)
				client.conn.Write(writeBuffer.Bytes())

			case RealtimeUpdateMsgType:
				if client.OnRealTimeUpdate != nil {
					realTimeUpdate, _ := unmarshalRealTimeUpdate(readBuffer)
					client.OnRealTimeUpdate(realTimeUpdate)
				}

			case RealtimeCarUpdateMsgType:
				if client.OnRealTimeCarUpdate != nil {
					carUpdate, _ := UnmarshalCarUpdateResp(readBuffer)
					client.OnRealTimeCarUpdate(carUpdate)
				}

			case EntryListMsgType:
				if client.OnEntryList != nil {
					_, carIds, _ := UnmarshalEntryListRep(readBuffer)
					client.OnEntryList(carIds)
				}

			case EntryListCarMsgType:
				if client.OnEntryListCar != nil {
					entryListCar, _ := UnmarshalEntryListCarResp(readBuffer)
					client.OnEntryListCar(entryListCar)
				}

			case TrackDataMsgType:
				if client.OnTrackData != nil {
					_, trackData, _ := UnmarshalTrackDataResp(readBuffer)
					client.OnTrackData(trackData)
				}

			case BroadcastingEventMsgType:
				if client.OnBroadCastEvent != nil {
					broadCastEvent, _ := unmarshalBroadCastEvent(readBuffer)
					client.OnBroadCastEvent(broadCastEvent)
				}

			default:
				log.Warn().Msg("WARNING:unrecognised msg-type")
			}
		}
	}
}

func (client *Client) Disconnect() {
	err := client.conn.Close()
	if err != nil {
		log.Warn().Msgf("WARNING:accbroadcastingsdk.Client: Error while disconnecting: %v", err)
	}
	if client.Wg != nil {
		client.Wg.Done()
	}
}
