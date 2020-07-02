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

	Championship string // to be transferred in session to router
	Event        string // to be transferred in session to router
	VideoFeed    string // to be transferred in session to router

	conn         *net.UDPConn // The UDP connection to ACC
	writeBuffer  bytes.Buffer // reusable buffer
	connectionId int32
}

func (client *Client) ConnectAndRun(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string, timeoutMs int32) {
	timeoutDuration := time.Duration(timeoutMs) * time.Millisecond
	attempt := 0

StartConnectionLoop:
	for true {
		if attempt > 0 {
			Logger.Info().Msg("Sleeping before retrying ...")
			time.Sleep(5 * time.Second)
		}
		attempt++

		Logger.Info().Msgf("Connecting to broadcasting interface at %s", address)

		raddr, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			Logger.Error().Msgf("resolving address:%v", err)
			continue StartConnectionLoop
		}

		client.conn, err = net.DialUDP("udp", nil, raddr)
		if err != nil {
			Logger.Error().Msgf("Error when establishing UDP connection: %v -> retrying", err)
		}

		MarshalConnectinReq(&client.writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
		client.conn.SetDeadline(time.Now().Add(timeoutDuration))
		n, err := client.conn.Write(client.writeBuffer.Bytes())
		if n < client.writeBuffer.Len() {
			Logger.Error().Msgf("Restarting connection because of connection request to broadcasting interface of ACC being partially written only")
			continue StartConnectionLoop
		}
		if err != nil {
			Logger.Error().Msgf("Restarting connection because of error while sending connection request to broadcasting interface of ACC: %v", err)
			continue StartConnectionLoop
		}

		var readArray [ReadBufferSize]byte
		for client.connectionId >= 0 {
			// read socket
			client.conn.SetDeadline(time.Now().Add(timeoutDuration))
			n, err = client.conn.Read(readArray[:])
			if err != nil {
				Logger.Error().Msgf("Retrying connection to broadcasting interface of ACC because of no response received after %dms.: '%v'", timeoutMs, err)
				continue StartConnectionLoop
			}
			if n == ReadBufferSize {
				Logger.Panic().Msg("Buffer not big enough !!!")
			}

			// extract msgType from first byte
			readBuffer := bytes.NewBuffer(readArray[:n])
			msgType, err := readBuffer.ReadByte()
			if err != nil {
				Logger.Error().Msgf("Restarting connection because of error reading the message-type: %v", err)
				continue StartConnectionLoop
			}

			// handle msg
			switch msgType {
			case RegistrationResultMsgType:
				Logger.Info().Msg("Recvd Registration")
				connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				client.connectionId = connectionId
				Logger.Info().Msgf("Connection: id:%d\tsuccess:%d\tread-only:%d\terr:'%s'", connectionId, connectionSuccess, isReadOnly, errMsg)

			case RealtimeUpdateMsgType:
				if client.OnRealTimeUpdate != nil {
					realTimeUpdate, _ := unmarshalRealTimeUpdate(readBuffer)
					client.OnRealTimeUpdate(realTimeUpdate)
				}

			case RealtimeCarUpdateMsgType:
				if client.OnRealTimeCarUpdate != nil {
					realTimeCarUpdate, _ := UnmarshalCarUpdateResp(readBuffer)
					client.OnRealTimeCarUpdate(realTimeCarUpdate)
				}

			case EntryListMsgType:
				if client.OnEntryList != nil {
					connectionId, entryList, ok := UnmarshalEntryListRep(readBuffer)
					Logger.Info().Msgf("EntryList (connection:%d;ok=%t): %v", connectionId, ok, entryList)
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

func (client *Client) RequestTrackData(connectionId int32) (ok bool) {
	client.writeBuffer.Reset()
	MarshalTrackDataReq(&client.writeBuffer, connectionId)
	n, err := client.conn.Write(client.writeBuffer.Bytes())
	if n != client.writeBuffer.Len() {
		Logger.Error().Msgf("Error while writing trackdata-req, wrote only %d bytes while it should have been %d", n, client.writeBuffer.Len())
		return false
	}
	if err != nil {
		Logger.Error().Msgf("Error while writing trackdata-req, %v", err)
		return false
	}
	return true
}

func (client *Client) RequestEntryList(connectionId int32) (ok bool) {
	client.writeBuffer.Reset()
	Logger.Info().Msgf("Requesting new entrylist (connectionId:%d)", connectionId)

	MarshalEntryListReq(&client.writeBuffer, connectionId)
	if !ok {
		Logger.Error().Msgf("Issue wehen marshaling entrlistreq")
		return false
	}

	n, err := client.conn.Write(client.writeBuffer.Bytes())
	Logger.Info().Msgf("Send new EntryList request for connection %d", connectionId)
	if n != client.writeBuffer.Len() {
		Logger.Error().Msgf("Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, client.writeBuffer.Len())
		return false
	}
	if err != nil {
		Logger.Error().Msgf("Error while writing entrylist-req, %v", err)
		return false
	}
	return true
}

func (client *Client) Disconnect() {
	client.writeBuffer.Reset()
	ok := MarshalDisconnectReq(&client.writeBuffer, client.connectionId)
	if !ok {
		Logger.Error().Msgf("Error when marhalling disconnecting %d", client.connectionId)
	}
	n, err := client.conn.Write(client.writeBuffer.Bytes())
	if n != client.writeBuffer.Len() {
		Logger.Error().Msgf("Error while writing disconnect, wrote only %d bytes while it should have been %d", n, client.writeBuffer.Len())
		return
	}
	if err != nil {
		Logger.Error().Msgf("Error while writing disconnect, %v", err)
		return
	}
	Logger.Info().Msgf("Disconnected %d", client.connectionId)
	client.connectionId = -1

	err = client.conn.Close()
	if err != nil {
		Logger.Warn().Msgf("WARNING:accbroadcastingsdk.Client: Error while disconnecting: %v", err)
	}
}
