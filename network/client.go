package network

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"net"
	"time"
)

const BROADCASTING_PROTOCOL_VERSION byte = 4
const ReadBufferSize = 32 * 1024

// At first connection, the callbacks are initially called in following order
//    - OnRealTimeUpdate
//    - OnRealTimeCarUpdate (for each car)
//    - OnEntryList
//    - OnEntryListCar (for each car)
//    - OnTrackData
//
// As of then at every refresh following are received (including when the session changes),
//    - OnRealTimeUpdate
//    - OnRealTimeCarUpdate (for each car)
//
// For events with a fixed entry-list, no entry-list updates are thus received anymore
//
// Since this interface is currently not sending broadcasting intructions, BroadCastEvent's are not received
type Client struct {
	conn                *net.UDPConn
	OnRealTimeUpdate    func(RealTimeUpdate)    // seems to be received first always when the connection is established
	OnRealTimeCarUpdate func(RealTimeCarUpdate) // seems to be received righ after RealTimeUpdate after the connection is established
	OnEntryList         func(EntryList)
	OnEntryListCar      func(EntryListCar)
	OnTrackData         func(TrackData)
	OnBroadCastEvent    func(BroadCastEvent)
}

func (client *Client) ConnectAndRun(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string, timeoutMs int32) {
	timeoutDuration := time.Duration(timeoutMs) * time.Millisecond
	attempt := 0

StartConnectionLoop:
	for true {
		if attempt > 0 {
			log.Info().Msg("Sleeping before retrying ...")
			time.Sleep(5 * time.Second)
		}
		attempt++

		log.Info().Msgf("Connecting to: %s", address)

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
		client.conn.SetDeadline(time.Now().Add(timeoutDuration))
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
		for !done {
			// read socket
			client.conn.SetDeadline(time.Now().Add(timeoutDuration))
			n, err = client.conn.Read(readArray[:])
			if err != nil {
				log.Error().Msgf("Error when reading message: '%v' -> restarting connection", err)
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
				connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				log.Info().Msgf("Connection: %d - %d - %d - %s", connectionId, connectionSuccess, isReadOnly, errMsg)

				writeBuffer.Reset()
				MarshalEntryListReq(&writeBuffer, connectionId)
				n, err = client.conn.Write(writeBuffer.Bytes())
				if n != writeBuffer.Len() {
					log.Error().Msgf("Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
					continue StartConnectionLoop
				}
				if err != nil {
					log.Error().Msgf("Error while writing entrylist-req, %v", err)
					continue StartConnectionLoop
				}

				writeBuffer.Reset()
				MarshalTrackDataReq(&writeBuffer, connectionId)
				n, err = client.conn.Write(writeBuffer.Bytes())
				if n != writeBuffer.Len() {
					log.Error().Msgf("Error while writing trackdata-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
					continue StartConnectionLoop
				}
				if err != nil {
					log.Error().Msgf("Error while writing trackdata-req, %v", err)
					continue StartConnectionLoop
				}

			case RealtimeUpdateMsgType:
				if client.OnRealTimeUpdate != nil {
					bufferLen := readBuffer.Len()
					if bufferLen < 81 {
						log.Error().Msgf("RealTimeUpdate only %d bytes (expected 81), dropping it", bufferLen)
						continue
					}
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
}
