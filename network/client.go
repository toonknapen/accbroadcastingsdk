package network

import (
	"bytes"
	"log"
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
	OnRealTimeUpdate    func(RealTimeUpdate)
	OnRealTimeCarUpdate func(RealTimeCarUpdate)
	OnBroadCastEvent    func(BroadCastEvent)
}

func (client *Client) ConnectAndRun(address string, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string) {
	attempt := 0

StartConnectionLoop:
	for true {
		if attempt > 0 {
			log.Println("Sleeping before retrying ...")
			time.Sleep(5 * time.Second)
		}
		attempt++

		log.Println("Connecting to:", address)

		raddr, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			log.Fatal("Fatal when resolving address:", err)
		}

		client.conn, err = net.DialUDP("udp", nil, raddr)
		if err != nil {
			log.Println("Error when establishing UDP connection:", err, "-> retrying")
		}

		var writeBuffer bytes.Buffer
		MarshalConnectinReq(&writeBuffer, displayName, connectionPassword, msRealtimeUpdateInterval, commandPassword)
		client.conn.SetDeadline(time.Now().Add(time.Second))
		n, err := client.conn.Write(writeBuffer.Bytes())
		if n < writeBuffer.Len() {
			log.Println("Error causing only to write partial message -> restarting connection")
			continue StartConnectionLoop
		}
		if err != nil {
			log.Println("Error when writing message-type:", err, "-> restarting connection")
			continue StartConnectionLoop
		}

		var readArray [ReadBufferSize]byte
		done := false
		for !done {
			// read socket
			client.conn.SetDeadline(time.Now().Add(time.Second))
			n, err = client.conn.Read(readArray[:])
			if err != nil {
				log.Println("Error when reading message-type:", err, "-> restarting connection")
				continue StartConnectionLoop
			}
			if n == ReadBufferSize {
				log.Fatal("Buffer not big enough !!!")
			}

			// extract msgType
			readBuffer := bytes.NewBuffer(readArray[:n])
			msgType, err := readBuffer.ReadByte()
			if err != nil {
				log.Println("No msgType -> restarting connection")
				continue StartConnectionLoop
			}

			// data
			// var sessionCarIds []uint16

			// handle msg
			switch msgType {
			case RegistrationResultMsgType:
				log.Println("Recvd Registration")
				connectionId, isReadOnly, errMsg, _ := UnmarshalConnectionResp(readBuffer)
				log.Println("Connection:", connectionId, isReadOnly, errMsg)

				writeBuffer.Reset()
				MarshalEntryListReq(&writeBuffer, connectionId)
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
				log.Println("Recvd TrackDataMsgType")

			case BroadcastingEventMsgType:
				if client.OnBroadCastEvent != nil {
					broadCastEvent, _ := unmarshalBroadCastEvent(readBuffer)
					client.OnBroadCastEvent(broadCastEvent)
				}

			default:
				log.Println("WARNING:unrecognised msg-type")
			}
		}
	}
}

func (client *Client) Disconnect() {
	err := client.conn.Close()
	if err != nil {
		log.Println("WARNING:accbroadcastingsdk.Client: Error while disconnecting", err)
	}
	if client.Wg != nil {
		client.Wg.Done()
	}
}
