package network

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"net"
	"time"
)

const BroadcastingProtocolVersion byte = 4
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
	OnRealTimeUpdate    func(RealTimeUpdate)    // seems to be received first always when the connection is established
	OnRealTimeCarUpdate func(RealTimeCarUpdate) // seems to be received righ after RealTimeUpdate after the connection is established

	// EntryList is only received on request from ACC
	// The client will request the entry-list each time a RealTimeCarUpdate is received for a car that is
	// not in the latest EntryList.
	OnEntryList func(EntryList)

	// An EntryListCar is received after having received the EntryList
	OnEntryListCar func(EntryListCar)

	OnTrackData      func(TrackData)
	OnBroadCastEvent func(BroadCastEvent)

	conn                 *net.UDPConn
	entryList            EntryList // nil until first entry-list is received
	lastEntryListRequest time.Time // do not ask more than once per sec
	carIdWarnedMissing   map[uint16]bool
}

func (client *Client) ConnectAndRun(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string, timeoutMs int32) {
	client.carIdWarnedMissing = make(map[uint16]bool)
	timeoutDuration := time.Duration(timeoutMs) * time.Millisecond
	attempt := 0
	var globalConnectionId int32

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

			// handle msg
			switch msgType {
			case RegistrationResultMsgType:
				log.Info().Msg("Recvd Registration")
				connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				globalConnectionId = connectionId
				log.Info().Msgf("Connection: id:%d\tsuccess:%d\tread-only:%d\terr:'%s'", connectionId, connectionSuccess, isReadOnly, errMsg)

				errorSendReqEntryList := client.sendReqEntryList(&writeBuffer, connectionId)
				if errorSendReqEntryList {
					log.Error().Msg("Error while sending req for entry-list, restarting connection")
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
					realTimeUpdate, _ := unmarshalRealTimeUpdate(readBuffer)
					client.OnRealTimeUpdate(realTimeUpdate)
				}

			case RealtimeCarUpdateMsgType:
				if client.entryList == nil {
					log.Info().Msgf("RealTimeCarUpdate not handled as entrylist not received yet")
				} else {
					if client.OnRealTimeCarUpdate != nil {
						realTimeCarUpdate, _ := UnmarshalCarUpdateResp(readBuffer)

						// check if car is known in entryList, otherwise ask for new entryList
						carId := realTimeCarUpdate.Id
						found := false
						for _, v := range client.entryList {
							if v == carId {
								found = true
								break
							}
						}

						if found {
							client.OnRealTimeCarUpdate(realTimeCarUpdate)
						} else {
							log.Info().Msgf("Car id %d unknown, fetching new entry-list for connection, %d", carId, globalConnectionId)
							client.entryList = nil
							error := client.sendReqEntryList(&writeBuffer, globalConnectionId)
							if error {
								log.Error().Msgf("Error when ")
							}
							//reportedMissing := client.carIdWarnedMissing[carId]
							//if !reportedMissing {
							//	log.Warn().Msgf("CarId %d is not in entry-list and will not be followed", carId)
							//	client.carIdWarnedMissing[carId] = true
							//}
						}
					}
				}

			case EntryListMsgType:
				if client.OnEntryList != nil {
					connectionId, entryList, ok := UnmarshalEntryListRep(readBuffer)
					log.Info().Msgf("EntryList (connection:%d;ok=%t): %v", connectionId, ok, entryList)
					client.entryList = entryList
					client.OnEntryList(entryList)
				}

			case EntryListCarMsgType:
				if client.OnEntryListCar != nil {
					entryListCar, _ := UnmarshalEntryListCarResp(readBuffer)
					log.Info().Msgf("EntryListCar: %+v", entryListCar)
					client.OnEntryListCar(entryListCar)
				}

			case TrackDataMsgType:
				if client.OnTrackData != nil {
					connectionId, trackData, ok := UnmarshalTrackDataResp(readBuffer)
					log.Info().Msgf("TrackData (connection:%d;ok=%t):%+v", connectionId, ok, trackData)
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

func (client *Client) sendReqEntryList(writeBuffer *bytes.Buffer, connectionId int32) (error bool) {
	writeBuffer.Reset()
	now := time.Now()
	if now.Sub(client.lastEntryListRequest) < time.Second {
		return
	}

	client.lastEntryListRequest = now
	ok := MarshalEntryListReq(writeBuffer, connectionId)
	if !ok {
		log.Error().Msgf("Issue wehen marshaling entrlistreq")
		return true
	}

	n, err := client.conn.Write(writeBuffer.Bytes())
	log.Info().Msgf("Send new EntryList request for connection %d", connectionId)
	if n != writeBuffer.Len() {
		log.Error().Msgf("Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		error = true
	}
	if err != nil {
		log.Error().Msgf("Error while writing entrylist-req, %v", err)
		error = true
	}
	return error
}

func (client *Client) Disconnect() {
	err := client.conn.Close()
	if err != nil {
		log.Warn().Msgf("WARNING:accbroadcastingsdk.Client: Error while disconnecting: %v", err)
	}
}
