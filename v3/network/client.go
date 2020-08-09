package network

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog/log"
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
	// Set and unset in ConnectAndListen
	conn *net.UDPConn

	timeOutDuration time.Duration

	// connectionId is received when being registered on the UDP interface.
	// At every subsequent request, the connectionId needs to be send along
	connectionId int32

	// stopListening can be set to true to stop the 'ConnectAndListen'
	stopListening bool
}

func (client *Client) ConnectAndListen(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string, timeoutMs int32) (success bool, errMsg string) {
	client.timeOutDuration = time.Duration(timeoutMs) * time.Millisecond

	success, errMsg = client.connect(address, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
	if success {
		success, errMsg = client.listen()
	}
	client.disconnect()

	return success, errMsg
}

func (client *Client) RequestTrackData() (ok bool) {
	if client.stopListening {
		return true
	}

	log.Debug().Msgf("ACCBroadCastAPI: Requesting track data (connectionId:%d)", client.connectionId)
	var writeBuffer bytes.Buffer
	MarshalTrackDataReq(&writeBuffer, client.connectionId)
	n, err := client.conn.Write(writeBuffer.Bytes())
	if n != writeBuffer.Len() {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing trackdata-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		return false
	}
	if err != nil {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing trackdata-req, %v", err)
		return false
	}
	return true
}

func (client *Client) RequestEntryList() (ok bool) {
	if client.stopListening {
		return true
	}

	log.Debug().Msgf("ACCBroadCastAPI: Requesting new entrylist (connectionId:%d)", client.connectionId)
	var writeBuffer bytes.Buffer
	MarshalEntryListReq(&writeBuffer, client.connectionId)
	n, err := client.conn.Write(writeBuffer.Bytes())
	log.Debug().Msgf("ACCBroadCastAPI: Send new EntryList request for connection %d", client.connectionId)
	if n != writeBuffer.Len() {
		log.Error().Msgf("ACCBroadCastAPI:Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		return false
	}
	if err != nil {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing entrylist-req, %v", err)
		return false
	}
	return true
}

func (client *Client) RequestDisconnect() {
	client.stopListening = true
}

func (client *Client) connect(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string) (success bool, errMsg string) {
	client.stopListening = false

	log.Info().Msgf("ACCBroadCastAPI: Connecting to %s", address)

	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		errMsg = fmt.Sprintf("ACCBroadCastAPI: error resolving address:%v", err)
		log.Error().Msg(errMsg)
		return false, errMsg
	}

	client.conn, err = net.DialUDP("udp", nil, raddr)
	if err != nil {
		errMsg = fmt.Sprintf("ACCBroadCastAPI: error resolving address:%v", err)
		log.Error().Msg(errMsg)
		return false, errMsg
	}

	var writeBuffer bytes.Buffer
	MarshalConnectinReq(&writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
	client.conn.SetDeadline(time.Now().Add(client.timeOutDuration))
	n, err := client.conn.Write(writeBuffer.Bytes())
	if n < writeBuffer.Len() {
		errMsg = fmt.Sprintf("ACCBroadCastAPI: Restarting connection because of connection request to broadcasting interface of ACC being partially written only")
		log.Error().Msg(errMsg)
		return false, errMsg
	}
	if err != nil {
		errMsg = fmt.Sprintf("ACCBroadCastAPI: Restarting connection because of error while sending connection request to broadcasting interface of ACC: %v", err)
		log.Error().Msg(errMsg)
		return false, errMsg
	}

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
			errMsg = fmt.Sprintf("ACCBroadCastAPI: ACC did not respond for %dms.: '%v'", client.timeOutDuration/time.Millisecond, err)
			log.Error().Msg(errMsg)
			break
		}
		if n == ReadBufferSize {
			log.Panic().Msg("ACCBroadCastAPI: Buffer not big enough !!!")
		}

		// extract msgType from first byte
		readBuffer := bytes.NewBuffer(readArray[:n])
		msgType, err := readBuffer.ReadByte()
		if err != nil {
			success = false
			client.stopListening = true
			errMsg = fmt.Sprintf("ACCBroadCastAPI: ACC message can not be interpreted: %v", err)
			log.Error().Msg(errMsg)
			break
		}

		// handle msg
		switch msgType {
		case RegistrationResultMsgType:
			log.Info().Msg("ACCBroadCastAPI: Recvd Registration")
			connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
			client.connectionId = connectionId
			log.Info().Msgf("ACCBroadCastAPI: Connection: id:%d, success:%d, read-only:%d, err:'%s'", connectionId, connectionSuccess, isReadOnly, errMsg)
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
				log.Debug().Msgf("ACCBroadCastAPI: EntryList (connection:%d;ok=%t): %v", connectionId, ok, entryList)
				client.OnEntryList(entryList)
			}

		case EntryListCarMsgType:
			if client.OnEntryListCar != nil {
				entryListCar, _ := UnmarshalEntryListCarResp(readBuffer)
				log.Debug().Msgf("ACCBroadCastAPI: EntryListCar: %+v", entryListCar)
				client.OnEntryListCar(entryListCar)
			}

		case TrackDataMsgType:
			if client.OnTrackData != nil {
				connectionId, trackData, ok := UnmarshalTrackDataResp(readBuffer)
				log.Debug().Msgf("ACCBroadCastAPI: TrackData (connection:%d;ok=%t):%+v", connectionId, ok, trackData)
				client.OnTrackData(trackData)
			}

		case BroadcastingEventMsgType:
			if client.OnBroadCastEvent != nil {
				broadCastEvent, _ := unmarshalBroadCastEvent(readBuffer)
				client.OnBroadCastEvent(broadCastEvent)
			}

		default:
			log.Warn().Msg("ACCBroadCastAPI: unrecognised msg-type")
		}
	}

	return success, errMsg
}

func (client *Client) disconnect() {
	var writeBuffer bytes.Buffer
	ok := MarshalDisconnectReq(&writeBuffer, client.connectionId)
	if !ok {
		log.Error().Msgf("ACCBroadCastAPI: Error when marhalling disconnecting %d", client.connectionId)
	}
	n, err := client.conn.Write(writeBuffer.Bytes())
	if n != writeBuffer.Len() {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing disconnect, wrote only %d bytes while it should have been %d", n, writeBuffer.Len())
		return
	}
	if err != nil {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing disconnect, %v", err)
		return
	}
	log.Info().Msgf("ACCBroadCastAPI: Disconnected %d was send", client.connectionId)

	err = client.conn.Close()
	if err != nil {
		log.Warn().Msgf("ACCBroadCastAPI: Error while disconnecting: %v", err)
	}
	client.conn = nil

	if client.OnDisconnected != nil {
		client.OnDisconnected()
	}
}

//func SetupCloseHandler(client *Client) {
//	c := make(chan os.Signal)
//	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
//	go func() {
//		<-c
//		log.Info().Msg("ACCBroadCastAPI: Ctrl-C pressed in Terminal, disconnecting from ACC")
//		client.Disconnect()
//		os.Exit(0)
//	}()
//}
