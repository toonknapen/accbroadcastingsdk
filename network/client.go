package network

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"net"
	"time"
)

const BroadcastingProtocolVersion byte = 4
const ReadBufferSize = 32 * 1024

var Logger = log.With().Str("component", "accbroadcastingsdk").Logger()

// After the connection is established, the OnRealTimeUpdate and OnRealTimeCarUpdate (for each car)
// will be called at the 'msRealTimeUpdateInterval`, the sample rate that is specified when connecting.
// Additionally OnBroadCastEvent will be called infrequently.
//
// When receiving confirmation that the connection is established, a request will be send for receiving
// the entry-list and the track-data. As a response to the request for the entry-list, OnEntryList
// and OnEntryListCar (for each car) will be called. As a response to the request for track-data, OnTrackData
// will be called.
//
// For coherency, OnRealTimeCarUpdate will only be called after OnEntryList and the corresponding OnEntryListCar
// have been called.
//
// Additionally whenever a car joins (and thus an update on that car without it being in the most recent entry-list),
// the OnRealCarUpdate is propagated. Instead a new request for the entry-list will be send and any onRealTimeCarUpdate's
// will only be received once the new entry-list is received and all the OnEntryListCar
type Client struct {
	OnRealTimeUpdate    func(RealTimeUpdate)    // called at every time sample
	OnRealTimeCarUpdate func(RealTimeCarUpdate) // called at every time sample once that car was received in the entry-list

	OnBroadCastEvent func(BroadCastEvent)

	// OnEntryList is only called after having received the entry-list at request.
	// The EntryList is requested at initial connection and every time a car is detected that was not in
	// the most recent OnEntryList
	OnEntryList func(EntryList)

	// OnEntryListCar is called for each car right after OnEntryList
	OnEntryListCar func(EntryListCar)

	// OnTrackData is only called after having received the track-data at request.
	// The TrackData is requested once the connection is established
	OnTrackData func(TrackData)

	// The UDP connection to ACC
	conn *net.UDPConn

	// Cache of last-received entry-list.
	// For every RealTimeCarUpdate will be verified if the carId was part of the most recent entry-list. If not,
	// the entry-list will be set back to nil and a request for a new entry-list will be submitted.
	entryList EntryList

	lastEntryListRequest time.Time // do not ask more than once per sec
}

func (client *Client) ConnectAndRun(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string, timeoutMs int32) {
	timeoutDuration := time.Duration(timeoutMs) * time.Millisecond
	attempt := 0
	var globalConnectionId int32

StartConnectionLoop:
	for true {
		if attempt > 0 {
			Logger.Info().Msg("Sleeping before retrying ...")
			time.Sleep(5 * time.Second)
		}
		attempt++

		Logger.Info().Msgf("Connecting to: %s", address)

		raddr, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			Logger.Error().Msgf("resolving address:%v", err)
			continue StartConnectionLoop
		}

		client.conn, err = net.DialUDP("udp", nil, raddr)
		if err != nil {
			Logger.Error().Msgf("Error when establishing UDP connection: %v -> retrying", err)
		}

		var writeBuffer bytes.Buffer
		MarshalConnectinReq(&writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
		client.conn.SetDeadline(time.Now().Add(timeoutDuration))
		n, err := client.conn.Write(writeBuffer.Bytes())
		if n < writeBuffer.Len() {
			Logger.Error().Msgf("Error causing only to write partial message -> restarting connection")
			continue StartConnectionLoop
		}
		if err != nil {
			Logger.Error().Msgf("Error when writing message-type: %v -> restarting connection", err)
			continue StartConnectionLoop
		}

		var readArray [ReadBufferSize]byte
		done := false
		for !done {
			// read socket
			client.conn.SetDeadline(time.Now().Add(timeoutDuration))
			n, err = client.conn.Read(readArray[:])
			if err != nil {
				Logger.Error().Msgf("Error when reading message: '%v' -> restarting connection", err)
				continue StartConnectionLoop
			}
			if n == ReadBufferSize {
				Logger.Panic().Msg("Buffer not big enough !!!")
			}

			// extract msgType
			readBuffer := bytes.NewBuffer(readArray[:n])
			msgType, err := readBuffer.ReadByte()
			if err != nil {
				Logger.Error().Msg("No msgType -> restarting connection")
				continue StartConnectionLoop
			}

			// handle msg
			switch msgType {
			case RegistrationResultMsgType:
				Logger.Info().Msg("Recvd Registration")
				connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				globalConnectionId = connectionId
				Logger.Info().Msgf("Connection: id:%d\tsuccess:%d\tread-only:%d\terr:'%s'", connectionId, connectionSuccess, isReadOnly, errMsg)

				errorSendReqEntryList := client.sendReqEntryList(&writeBuffer, connectionId)
				if errorSendReqEntryList {
					Logger.Error().Msg("Error while sending req for entry-list, restarting connection")
					continue StartConnectionLoop
				}

				writeBuffer.Reset()
				MarshalTrackDataReq(&writeBuffer, connectionId)
				n, err = client.conn.Write(writeBuffer.Bytes())
				if n != writeBuffer.Len() {
					Logger.Error().Msgf("Error while writing trackdata-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
					continue StartConnectionLoop
				}
				if err != nil {
					Logger.Error().Msgf("Error while writing trackdata-req, %v", err)
					continue StartConnectionLoop
				}

			case RealtimeUpdateMsgType:
				if client.OnRealTimeUpdate != nil {
					realTimeUpdate, _ := unmarshalRealTimeUpdate(readBuffer)
					client.OnRealTimeUpdate(realTimeUpdate)
				}

			case RealtimeCarUpdateMsgType:
				if client.entryList == nil {
					Logger.Info().Msgf("RealTimeCarUpdate not handled as entrylist not received yet")
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
							Logger.Info().Msgf("Car id %d unknown, fetching new entry-list for connection, %d", carId, globalConnectionId)
							client.entryList = nil
							error := client.sendReqEntryList(&writeBuffer, globalConnectionId)
							if error {
								Logger.Error().Msgf("Error when ")
							}
						}
					}
				}

			case EntryListMsgType:
				if client.OnEntryList != nil {
					connectionId, entryList, ok := UnmarshalEntryListRep(readBuffer)
					Logger.Info().Msgf("EntryList (connection:%d;ok=%t): %v", connectionId, ok, entryList)
					client.entryList = entryList
					client.OnEntryList(entryList)
				}

			case EntryListCarMsgType:
				if client.OnEntryListCar != nil {
					entryListCar, _ := UnmarshalEntryListCarResp(readBuffer)
					Logger.Info().Msgf("EntryListCar: %+v", entryListCar)
					client.OnEntryListCar(entryListCar)
				}

			case TrackDataMsgType:
				if client.OnTrackData != nil {
					connectionId, trackData, ok := UnmarshalTrackDataResp(readBuffer)
					Logger.Info().Msgf("TrackData (connection:%d;ok=%t):%+v", connectionId, ok, trackData)
					client.OnTrackData(trackData)
				}

			case BroadcastingEventMsgType:
				if client.OnBroadCastEvent != nil {
					broadCastEvent, _ := unmarshalBroadCastEvent(readBuffer)
					client.OnBroadCastEvent(broadCastEvent)
				}

			default:
				Logger.Warn().Msg("WARNING:unrecognised msg-type")
			}
		}
	}
}

func (client *Client) sendReqEntryList(writeBuffer *bytes.Buffer, connectionId int32) (error bool) {
	writeBuffer.Reset()
	Logger.Info().Msgf("accbroadcastingsdk: ")
	now := time.Now()
	if now.Sub(client.lastEntryListRequest) < time.Second {
		return
	}

	client.lastEntryListRequest = now
	ok := MarshalEntryListReq(writeBuffer, connectionId)
	if !ok {
		Logger.Error().Msgf("Issue wehen marshaling entrlistreq")
		return true
	}

	n, err := client.conn.Write(writeBuffer.Bytes())
	Logger.Info().Msgf("Send new EntryList request for connection %d", connectionId)
	if n != writeBuffer.Len() {
		Logger.Error().Msgf("Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		error = true
	}
	if err != nil {
		Logger.Error().Msgf("Error while writing entrylist-req, %v", err)
		error = true
	}
	return error
}

func (client *Client) Disconnect() {
	err := client.conn.Close()
	if err != nil {
		Logger.Warn().Msgf("WARNING:accbroadcastingsdk.Client: Error while disconnecting: %v", err)
	}
}
