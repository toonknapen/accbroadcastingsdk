package network

import (
	"bytes"
	"encoding/binary"
	"log"
)

type OutboundMessageTypes = byte

const (
	RegisterCommandApplication OutboundMessageTypes = 1
	// UNREGISTER_COMMAND_APPLICATION OutboundMessageTypes = 9
	RequestEntryList OutboundMessageTypes = 10
	// REQUEST_TRACK_DATA             OutboundMessageTypes = 11
	// CHANGE_HUD_PAGE                OutboundMessageTypes = 49
	// CHANGE_FOCUS                   OutboundMessageTypes = 50
	// INSTANT_REPLAY_REQUEST         OutboundMessageTypes = 51
)

type InboundMessageTypes = byte

const (
	RegistrationResultMsgType InboundMessageTypes = 1
	RealtimeUpdateMsgType     InboundMessageTypes = 2
	RealtimeCarUpdateMsgType  InboundMessageTypes = 3
	EntryListMsgType          InboundMessageTypes = 4
	EntryListCarMsgType       InboundMessageTypes = 6
	TrackDataMsgType          InboundMessageTypes = 5
	BroadcastingEventMsgType  InboundMessageTypes = 7
)

type EntryList []uint16

type EntryListCar struct {
	id              uint16
	model           byte
	teamName        string
	raceNumber      int32
	cupCategory     byte
	currentDriverId int8
	drivers         []Driver
}

type CarUpdate struct {
	Id             uint16
	DriverId       uint16
	DriverCount    uint8
	Gear           int8
	WorldPosX      float32
	WorldPosY      float32
	Yaw            float32
	CarLocation    uint8
	Kmh            uint16
	Position       uint16
	CupPosition    uint16
	TrackPosition  uint16
	SplinePosition float32
	Laps           uint16
	Delta          int32
	BestSessionLap Lap
	LastLap        Lap
	CurrentLap     Lap
}

type Lap struct {
	LapTimeMs      int32
	CarId          uint16
	DriverId       uint16
	Splits         []int32
	IsInvalid      byte
	IsValidForBest byte
	IsOutLap       byte
	IsInLap        byte
}

type Driver struct {
	firstName string
	lastName  string
	shortName string
	category  byte
}

func MarshalConnectinReq(buffer *bytes.Buffer, displayName string, connectionPassword string, msRealtimeUpdateInterval int32, commandPassword string) (ok bool) {
	ok = writeByteBuffer(buffer, RegisterCommandApplication)
	ok = ok && writeByteBuffer(buffer, BROADCASTING_PROTOCOL_VERSION)
	ok = ok && writeString(buffer, displayName)
	ok = ok && writeString(buffer, connectionPassword)
	ok = ok && writeBuffer(buffer, msRealtimeUpdateInterval)
	ok = ok && writeString(buffer, commandPassword)
	return ok
}

func UnmarshalConnectionResp(buffer *bytes.Buffer) (connectionId int32, isReadOnly int8, errMsg string, ok bool) {
	ok = readBuffer(buffer, &connectionId)
	ok = ok && readBuffer(buffer, &isReadOnly)
	ok = ok && readString(buffer, &errMsg)
	return connectionId, isReadOnly, errMsg, ok
}

func MarshalEntryListReq(buffer *bytes.Buffer, connectionId int32) bool {
	ok := writeByteBuffer(buffer, RequestEntryList)
	ok = ok && writeBuffer(buffer, connectionId)
	return ok
}

func UnmarshalEntryListRep(buffer *bytes.Buffer) (connectionId int32, carIds EntryList, ok bool) {
	ok = readBuffer(buffer, &connectionId)
	var entryCount uint16
	ok = ok && readBuffer(buffer, &entryCount)
	carIds = make(EntryList, entryCount)
	for i := uint16(0); ok && i < entryCount; i++ {
		ok = ok && readBuffer(buffer, &carIds[i])
	}
	return connectionId, carIds, ok
}

func UnmarshalEntryListCarResp(buffer *bytes.Buffer) (car EntryListCar, ok bool) {
	ok = readBuffer(buffer, &car.id)
	ok = ok && readBuffer(buffer, &car.model)
	ok = ok && readString(buffer, &car.teamName)
	ok = ok && readBuffer(buffer, &car.raceNumber)
	ok = ok && readBuffer(buffer, &car.cupCategory)
	ok = ok && readBuffer(buffer, &car.currentDriverId)

	var driversOnCarCount uint8
	ok = ok && readBuffer(buffer, &driversOnCarCount)
	car.drivers = make([]Driver, driversOnCarCount)
	for i := uint8(0); ok && i < driversOnCarCount; i++ {
		ok = ok && readString(buffer, &car.drivers[i].firstName)
		ok = ok && readString(buffer, &car.drivers[i].lastName)
		ok = ok && readString(buffer, &car.drivers[i].shortName)
		ok = ok && readBuffer(buffer, &(car.drivers[i].category))
	}
	return car, ok
}

func UnmarshalLap(buffer *bytes.Buffer) (lap Lap, ok bool) {
	ok = readBuffer(buffer, &lap.LapTimeMs)
	ok = ok && readBuffer(buffer, &lap.CarId)
	ok = ok && readBuffer(buffer, &lap.DriverId)

	var splitCount uint8
	ok = ok && readBuffer(buffer, &splitCount)
	lap.Splits = make([]int32, splitCount)
	for i := uint8(0); ok && i < splitCount; i++ {
		ok = ok && readBuffer(buffer, &(lap.Splits[i]))
	}
	ok = ok && readBuffer(buffer, &lap.IsInvalid)
	ok = ok && readBuffer(buffer, &lap.IsValidForBest)
	ok = ok && readBuffer(buffer, &lap.IsOutLap)
	ok = ok && readBuffer(buffer, &lap.IsInLap)
	return lap, ok
}

func UnmarshalCarUpdateResp(buffer *bytes.Buffer) (carUpdate CarUpdate, ok bool) {
	ok = readBuffer(buffer, &carUpdate.Id)
	ok = ok && readBuffer(buffer, &carUpdate.DriverId)
	ok = ok && readBuffer(buffer, &carUpdate.DriverCount)
	ok = ok && readBuffer(buffer, &carUpdate.Gear)
	ok = ok && readBuffer(buffer, &carUpdate.WorldPosX)
	ok = ok && readBuffer(buffer, &carUpdate.WorldPosY)
	ok = ok && readBuffer(buffer, &carUpdate.Yaw)
	ok = ok && readBuffer(buffer, &carUpdate.CarLocation)
	ok = ok && readBuffer(buffer, &carUpdate.Kmh)
	ok = ok && readBuffer(buffer, &carUpdate.Position)
	ok = ok && readBuffer(buffer, &carUpdate.CupPosition)
	ok = ok && readBuffer(buffer, &carUpdate.TrackPosition)
	ok = ok && readBuffer(buffer, &carUpdate.SplinePosition)
	ok = ok && readBuffer(buffer, &carUpdate.Laps)
	ok = ok && readBuffer(buffer, &carUpdate.Delta)
	if ok {
		carUpdate.BestSessionLap, ok = UnmarshalLap(buffer)
	}
	if ok {
		carUpdate.LastLap, ok = UnmarshalLap(buffer)
	}
	if ok {
		carUpdate.CurrentLap, ok = UnmarshalLap(buffer)
	}
	return carUpdate, ok
}

func writeByteBuffer(buffer *bytes.Buffer, b byte) bool {
	err := buffer.WriteByte(b)
	if err != nil {
		log.Println("Error in writeBuffer:", err)
		return false
	}
	return true
}

func writeBuffer(buffer *bytes.Buffer, data interface{}) bool {
	err := binary.Write(buffer, binary.LittleEndian, data)
	if err != nil {
		log.Println("Error in writeBuffer:", err)
		return false
	}
	return true
}

func readBuffer(buffer *bytes.Buffer, data interface{}) bool {
	err := binary.Read(buffer, binary.LittleEndian, data)
	if err != nil {
		log.Println("Error in readBuffer:", err)
		return false
	}
	return true
}

func writeString(buffer *bytes.Buffer, s string) bool {
	length := int16(len(s))
	err := binary.Write(buffer, binary.LittleEndian, length)
	if err != nil {
		log.Println("Error in writeString:", err)
		return false
	}
	buffer.Write([]byte(s))
	return true
}

func readString(buffer *bytes.Buffer, s *string) bool {
	var length int16
	err := binary.Read(buffer, binary.LittleEndian, &length)
	if err != nil {
		log.Println("Error in readString:", err)
		return false
	}
	stringBuffer := make([]byte, length)
	err = binary.Read(buffer, binary.LittleEndian, &stringBuffer)
	if err != nil {
		log.Println("Error while reading in readStr:", err)
		return false
	}
	*s = string(stringBuffer)
	return true
}
