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
	RegistrationResult InboundMessageTypes = 1
	RealtimeUpdate     InboundMessageTypes = 2
	RealtimeCarUpdate  InboundMessageTypes = 3
	EntryList          InboundMessageTypes = 4
	EntryListCar       InboundMessageTypes = 6
	TrackData          InboundMessageTypes = 5
	BroadcastingEvent  InboundMessageTypes = 7
)

type Driver struct {
	firstName string
	lastName  string
	shortName string
	category  byte
}

type Car struct {
	id              uint16
	model           byte
	teamName        string
	raceNumber      int32
	cupCategory     byte
	currentDriverId int8
	drivers         []Driver
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

func UnmarshalEntryListRep(buffer *bytes.Buffer) (connectionId int32, carIds []uint16, ok bool) {
	ok = readBuffer(buffer, &connectionId)
	var entryCount uint16
	ok = ok && readBuffer(buffer, &entryCount)
	carIds = make([]uint16, entryCount)
	for i := uint16(0); ok && i < entryCount; i++ {
		ok = ok && readBuffer(buffer, &carIds[i])
	}
	return connectionId, carIds, ok
}

func UnmarshalEntryListCarResp(buffer *bytes.Buffer) (car Car, ok bool) {
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

func UnmarshalLap(buffer *bytes.Buffer) (lap Lap) {
	binary.Read(buffer, binary.LittleEndian, &lap.LapTimeMs)
	binary.Read(buffer, binary.LittleEndian, &lap.CarId)
	binary.Read(buffer, binary.LittleEndian, &lap.DriverId)

	var splitCount uint8
	binary.Read(buffer, binary.LittleEndian, &splitCount)
	lap.Splits = make([]int32, splitCount)
	for i := uint8(0); i < splitCount; i++ {
		binary.Read(buffer, binary.LittleEndian, &(lap.Splits[i]))
	}
	binary.Read(buffer, binary.LittleEndian, &lap.IsInvalid)
	binary.Read(buffer, binary.LittleEndian, &lap.IsValidForBest)
	binary.Read(buffer, binary.LittleEndian, &lap.IsOutLap)
	binary.Read(buffer, binary.LittleEndian, &lap.IsInLap)
	return lap
}

func UnmarshalCarUpdateResp(buffer *bytes.Buffer) (carUpdate CarUpdate) {
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Id)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.DriverId)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.DriverCount)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Gear)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.WorldPosX)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.WorldPosY)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Yaw)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.CarLocation)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Kmh)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Position)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.CupPosition)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.TrackPosition)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.SplinePosition)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Laps)
	binary.Read(buffer, binary.LittleEndian, &carUpdate.Delta)
	carUpdate.BestSessionLap = UnmarshalLap(buffer)
	carUpdate.LastLap = UnmarshalLap(buffer)
	carUpdate.CurrentLap = UnmarshalLap(buffer)
	return carUpdate
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
