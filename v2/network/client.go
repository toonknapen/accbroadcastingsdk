package network

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"github.com/toonknapen/accbroadcastingsdk/network"
	"net"
	"os"
	"os/signal"
	"syscall"
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
			log.Info().Msg("ACCBroadCastAPI: Sleeping before retrying ...")
			time.Sleep(5 * time.Second)
		}
		attempt++

		log.Info().Msgf("ACCBroadCastAPI: Connecting to %s", address)

		raddr, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			log.Error().Msgf("ACCBroadCastAPI: error resolving address:%v", err)
			continue StartConnectionLoop
		}

		client.conn, err = net.DialUDP("udp", nil, raddr)
		if err != nil {
			log.Error().Msgf("ACCBroadCastAPI: Retrying connection due to error when establishing UDP connection: %v", err)
		}

		MarshalConnectinReq(&client.writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
		client.conn.SetDeadline(time.Now().Add(timeoutDuration))
		n, err := client.conn.Write(client.writeBuffer.Bytes())
		if n < client.writeBuffer.Len() {
			log.Error().Msgf("ACCBroadCastAPI: Restarting connection because of connection request to broadcasting interface of ACC being partially written only")
			continue StartConnectionLoop
		}
		if err != nil {
			log.Error().Msgf("ACCBroadCastAPI: Restarting connection because of error while sending connection request to broadcasting interface of ACC: %v", err)
			continue StartConnectionLoop
		}

		var readArray [ReadBufferSize]byte
		for client.connectionId >= 0 {
			// read socket
			client.conn.SetDeadline(time.Now().Add(timeoutDuration))
			n, err = client.conn.Read(readArray[:])
			if err != nil {
				log.Error().Msgf("ACCBroadCastAPI: Retrying connection to broadcasting interface of ACC because of no response received after %dms.: '%v'", timeoutMs, err)
				continue StartConnectionLoop
			}
			if n == ReadBufferSize {
				log.Panic().Msg("ACCBroadCastAPI: Buffer not big enough !!!")
			}

			// extract msgType from first byte
			readBuffer := bytes.NewBuffer(readArray[:n])
			msgType, err := readBuffer.ReadByte()
			if err != nil {
				log.Error().Msgf("ACCBroadCastAPI: Restarting connection because of error reading the message-type: %v", err)
				continue StartConnectionLoop
			}

			// handle msg
			switch msgType {
			case RegistrationResultMsgType:
				log.Info().Msg("ACCBroadCastAPI: Recvd Registration")
				connectionId, connectionSuccess, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				client.connectionId = connectionId
				log.Info().Msgf("ACCBroadCastAPI: Connection: id:%d, success:%d, read-only:%d, err:'%s'", connectionId, connectionSuccess, isReadOnly, errMsg)

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
	}
}

func (client *Client) RequestTrackData() (ok bool) {
	log.Debug().Msgf("ACCBroadCastAPI: Requesting track data (connectionId:%d)", client.connectionId)
	client.writeBuffer.Reset()
	MarshalTrackDataReq(&client.writeBuffer, client.connectionId)
	n, err := client.conn.Write(client.writeBuffer.Bytes())
	if n != client.writeBuffer.Len() {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing trackdata-req, wrote only %d bytes while it should have been %d", n, client.writeBuffer.Len())
		return false
	}
	if err != nil {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing trackdata-req, %v", err)
		return false
	}
	return true
}

func (client *Client) RequestEntryList() (ok bool) {
	log.Debug().Msgf("ACCBroadCastAPI: Requesting new entrylist (connectionId:%d)", client.connectionId)
	client.writeBuffer.Reset()
	MarshalEntryListReq(&client.writeBuffer, client.connectionId)
	n, err := client.conn.Write(client.writeBuffer.Bytes())
	log.Debug().Msgf("ACCBroadCastAPI: Send new EntryList request for connection %d", client.connectionId)
	if n != client.writeBuffer.Len() {
		log.Error().Msgf("ACCBroadCastAPI:Error while writing entrylist-req, wrote only %d bytes while it should have been %d", n, client.writeBuffer.Len())
		return false
	}
	if err != nil {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing entrylist-req, %v", err)
		return false
	}
	return true
}

func (client *Client) Disconnect() {
	client.writeBuffer.Reset()
	ok := MarshalDisconnectReq(&client.writeBuffer, client.connectionId)
	if !ok {
		log.Error().Msgf("ACCBroadCastAPI: Error when marhalling disconnecting %d", client.connectionId)
	}
	n, err := client.conn.Write(client.writeBuffer.Bytes())
	if n != client.writeBuffer.Len() {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing disconnect, wrote only %d bytes while it should have been %d", n, client.writeBuffer.Len())
		return
	}
	if err != nil {
		log.Error().Msgf("ACCBroadCastAPI: Error while writing disconnect, %v", err)
		return
	}
	log.Info().Msgf("ACCBroadCastAPI: Disconnected %d", client.connectionId)
	client.connectionId = -1

	err = client.conn.Close()
	if err != nil {
		log.Warn().Msgf("ACCBroadCastAPI: Error while disconnecting: %v", err)
	}
}

func SetupCloseHandler(client *Client) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msg("ACCBroadCastAPI: Ctrl-C pressed in Terminal, disconnecting from ACC")
		client.Disconnect()
		os.Exit(0)
	}()
}
