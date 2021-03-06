package network

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog"
	"net"
	"time"
)

const BroadcastingProtocolVersion byte = 4
const ReadBufferSize = 32 * 1024

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
	Logger zerolog.Logger

	OnConnected    func(connectionId int32)
	OnDisconnected func()

	// OnRealTimeUpdate is called at every time sample
	OnRealTimeUpdate func(RealTimeUpdate)

	// OnRealTimeCarUpdate is called at every time sample.
	// It might contain an update for a car that was not in the last received entryList
	OnRealTimeCarUpdate func(RealTimeCarUpdate)

	//
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

	// conn is the UDP connection to ACC
	// Set and unset in ConnectListenAndCallback
	conn *net.UDPConn

	timeOutDuration time.Duration

	// connectionId is received when being registered on the UDP interface.
	// At every subsequent request, the connectionId needs to be send along
	connectionId int32

	// stopListening can be set to true to stop the 'ConnectListenAndCallback'
	stopListening bool
}

// ConnectListenAndCallback will connect to the ACC UDP broadcasting interface and call the corresponding callback for
// each data element that is received.
//
// When trying to connect or while being connected and nothing was received within the 'timeoutMs' interval,
// the connection will be considered broken and this function will return.
//
// To stop listening to the UDP interface, `RequestDisconnect()` can be called. This function will attempt to
// disconnect from the UDP interface (as to be able to reconnect again) before returning. Note that it might
// take 'timeoutMs' before the disconnect will be send to ACC after the execution of RequestDisconnect.
func (client *Client) ConnectListenAndCallback(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string, timeoutMs int32) (success bool, errMsg string) {
	client.timeOutDuration = time.Duration(timeoutMs) * time.Millisecond

	success, errMsg = client.connect(address, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)

	if success {
		success, errMsg = client.listen()
	}
	client.disconnect()

	client.Logger.Info().Msgf("ACC client stopped listening and disconnected")
	return success, errMsg
}

func (client *Client) RequestTrackData() (ok bool) {
	if client.stopListening {
		return true
	}

	client.Logger.Debug().Msgf("Requesting track data (connectionId:%d)", client.connectionId)
	var writeBuffer bytes.Buffer
	MarshalTrackDataReq(&writeBuffer, client.connectionId)
	n, err := client.conn.Write(writeBuffer.Bytes())
	if n != writeBuffer.Len() {
		client.Logger.Error().Msgf("Error while writing trackdata-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		return false
	}
	if err != nil {
		client.Logger.Error().Msgf("Error while writing trackdata-req, %v", err)
		return false
	}
	return true
}

func (client *Client) RequestEntryList() (ok bool) {
	if client.stopListening {
		return true
	}

	client.Logger.Debug().Msgf("Requesting new entrylist (connectionId:%d)", client.connectionId)
	var writeBuffer bytes.Buffer
	MarshalEntryListReq(&writeBuffer, client.connectionId)
	n, err := client.conn.Write(writeBuffer.Bytes())
	client.Logger.Debug().Msgf("Send new EntryList request for connection %d", client.connectionId)
	if n != writeBuffer.Len() {
		client.Logger.Error().Msgf("Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		return false
	}
	if err != nil {
		client.Logger.Error().Msgf("Error while writing entrylist-req, %v", err)
		return false
	}
	return true
}

func (client *Client) RequestDisconnect() {
	client.stopListening = true
}

func (client *Client) connect(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string) (success bool, errMsg string) {
	client.stopListening = false

	client.Logger.Info().Msgf("Connecting to %s", address)

	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		client.Logger.Error().Int(Code, ErrorAddressNotResolved).Msgf("error resolving address:%v", err)
		return false, errMsg
	}

	client.conn, err = net.DialUDP("udp", nil, raddr)
	if err != nil {
		client.Logger.Error().Int(Code, ErrorSetupUDPConnection).Msgf("error resolving address:%v", err)
		return false, errMsg
	}

	var writeBuffer bytes.Buffer
	MarshalRegistrationReq(&writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
	client.conn.SetDeadline(time.Now().Add(client.timeOutDuration))
	n, err := client.conn.Write(writeBuffer.Bytes())
	if n < writeBuffer.Len() {
		errMsg = fmt.Sprintf("registration request partially written only")
		client.Logger.Error().Msg(errMsg)
		return false, errMsg
	}
	if err != nil {
		errMsg = fmt.Sprintf("error while writing registration request to ACC: %v", err)
		client.Logger.Error().Msg(errMsg)
		return false, errMsg
	}

	client.Logger.Info().Int(Code, InfoRegistrationReqSendToAcc).Msgf("Registration request send to ACC")
	return true, ""
}

func (client *Client) listen() (success bool, errMsg string) {
	success = true
	var readArray [ReadBufferSize]byte

	for !client.stopListening {
		// read socket
		client.conn.SetDeadline(time.Now().Add(client.timeOutDuration))
		n, err := client.conn.Read(readArray[:])
		if err != nil {
			success = false
			client.stopListening = true
			client.Logger.Error().Int(Code, ErrorReadTimeout).Msgf("ACC did not respond for %dms.: '%v'", client.timeOutDuration/time.Millisecond, err)
			break
		}
		if n == ReadBufferSize {
			client.Logger.Panic().Msg("Buffer not big enough !!!")
		}

		// extract msgType from first byte
		readBuffer := bytes.NewBuffer(readArray[:n])
		msgType, err := readBuffer.ReadByte()
		if err != nil {
			success = false
			client.stopListening = true
			client.Logger.Error().Msgf("ACC message can not be interpreted: %v", err)
			break
		}

		// handle msg
		switch msgType {
		case RegistrationResultMsgType:
			client.Logger.Info().Msg("Recvd Registration")
			connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
			client.connectionId = connectionId
			client.Logger.Info().Int(Code, InfoRegistrationAckByAcc).Msgf("Connection: id:%d, success:%d, read-only:%d, err:'%s'", connectionId, connectionSuccess, isReadOnly, errMsg)
			if client.OnConnected != nil {
				client.OnConnected(client.connectionId)
			}

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
				client.Logger.Debug().Msgf("EntryList (connection:%d;ok=%t): %v", connectionId, ok, entryList)
				client.OnEntryList(entryList)
			}

		case EntryListCarMsgType:
			if client.OnEntryListCar != nil {
				entryListCar, _ := UnmarshalEntryListCarResp(readBuffer)
				client.Logger.Debug().Msgf("EntryListCar: %+v", entryListCar)
				client.OnEntryListCar(entryListCar)
			}

		case TrackDataMsgType:
			if client.OnTrackData != nil {
				connectionId, trackData, ok := UnmarshalTrackDataResp(readBuffer)
				client.Logger.Debug().Msgf("TrackData (connection:%d;ok=%t):%+v", connectionId, ok, trackData)
				client.OnTrackData(trackData)
			}

		case BroadcastingEventMsgType:
			if client.OnBroadCastEvent != nil {
				broadCastEvent, _ := unmarshalBroadCastEvent(readBuffer)
				client.OnBroadCastEvent(broadCastEvent)
			}

		default:
			client.Logger.Warn().Msg("unrecognised msg-type")
		}
	}

	return success, errMsg
}

func (client *Client) disconnect() {
	var writeBuffer bytes.Buffer
	ok := MarshalDisconnectReq(&writeBuffer, client.connectionId)
	if !ok {
		client.Logger.Error().Msgf("Error when marhalling disconnecting %d", client.connectionId)
	}
	n, err := client.conn.Write(writeBuffer.Bytes())
	if n != writeBuffer.Len() {
		client.Logger.Error().Msgf("Error while writing disconnect, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		return
	}
	if err != nil {
		client.Logger.Error().Msgf("Error while writing disconnect, %v", err)
		return
	}
	client.Logger.Info().Msgf("Disconnected %d was send", client.connectionId)

	err = client.conn.Close()
	if err != nil {
		client.Logger.Warn().Msgf("Error while disconnecting: %v", err)
	}
	client.conn = nil

	if client.OnDisconnected != nil {
		client.OnDisconnected()
	}
}
