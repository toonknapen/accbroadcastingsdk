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
	RequestTrackData OutboundMessageTypes = 11
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

const (
	SessionTypePractice        = 0
	SessionTypeQualifying      = 4
	SessionTypeSuperpole       = 9
	SessionTypeRace            = 10
	SessionTypeHotlap          = 11
	SessionTypeHotstint        = 12
	SessionTypeHotlapSuperpole = 13
	SessionTypeReplay          = 14
)

const (
	SessionPhaseNONE         = 0
	SessionPhaseStarting     = 1
	SessionPhasePreFormation = 2
	SessionPhaseFormationLap = 3
	SessionPhasePreSession   = 4 // during formation-lap
	SessionPhaseSession      = 5 // as of green light
	SessionPhaseSessionOver  = 6
	SessionPhasePostSession  = 7
	SessionPhaseResultUI     = 8
)

const (
	CarLocationNONE    = 0
	CarLocationTrack   = 1
	CarLocationPitlane = 2

	// The location just becomes briefly CarLocationPitEntry and then afterwards becomes CarLocationPitLane.
	// toFigureOut: How long exactly is the location CarLocationPitEntry and how to determine the exact time of the Pit-in
	CarLocationPitEntry = 3

	CarLocationPitExit = 4
)

const (
	CarModelMercedes    = 1
	CarModelFerrari     = 2
	CarModelLexus       = 15
	CarModelLamborghini = 16
	CarModelAudi        = 19
	CarModelAstonMartin = 20
	CarModelPorsche     = 23
)

const (
	TrackNameBrandsHatch = "Brands Hatch Circuit"
	TrackNameSpa         = "Circuit de Spa-Francorchamps"
	TrackNameMonza       = "Monza Circuit"
	TrackNameMisano      = "Misano World Circuit"
	TrackNamePaulRicard  = "Circuit Paul Ricard"
	TrackNameSilversone  = "Silverstone"
	TrackNameHungaroring = "Hungaroring"
	TrackNameNurburgring = "Nürburgring"
	TrackNameBarcelona   = "Circuit de Barcelona-Catalunya"
	TrackNameZolder      = "Circuit Zolder"
	TrackNameZandvoort   = "Circuit Zandvoort"
)

const (
	TrackIdBrandsHatch = 1
	TrackIdSpa         = 2
	TrackIdMonza       = 3
	TrackIdMisano      = 4
	TrackIdPaulRicard  = 5
	TrackIdSilverstone = 6
	TrackIdHungaroring = 7
	TrackIdNurburgring = 8
	TrackIdBarcelona   = 9
	TrackIdZolder      = 10
	TrackIdZandvoort   = 11
)

// EntryList provides an array of internal id's of each car in the session
//
// This id is used when sending car-info using the `EntryListCar` structure. These id's seem to be always
// 0-based and incrementing sequentially (thus [0, 1, 2, 3, 4, 5, ... n-1])
type EntryList []uint16

type EntryListCar struct {
	Id              uint16 // Id that was already communicated in the EntryList
	Model           byte   // One of constants CarModel<name>
	TeamName        string
	RaceNumber      int32 // the number shown on the car-body and in the leaderboard
	CupCategory     byte
	CurrentDriverId int8
	Drivers         []Driver
}

type TrackData struct {
	Name   string // Will be equal to one of the constants TrackName<name>
	Id     int32  // Will be equal to one of the constants TrackId<name>
	Meters int32
}

type RealTimeUpdate struct {
	EventIndex      uint16  // >= 0
	SessionIndex    uint16  // >= 0
	SessionType     byte    // see SessionType<name> constants
	Phase           byte    // see SessionPhase<name> constants
	SessionTime     float32 // ms since session started (green light)
	SessionEndTime  float32 // remaining duration of current session in ms
	FocusedCarIndex int32
	ActiveCameraSet string
	ActiveCamera    string
	CurrentHUDPage  string
	IsReplayPlaying byte // yes is != 0x00
	TimeOfDay       float32
	AmbientTemp     int8
	TrackTemp       int8
	Clouds          byte
	RainLevel       byte
	Wettness        byte
	BestSessionLap  Lap
}

type RealTimeCarUpdate struct {
	Id             uint16  // Id of one of the cars in the EntryList and thus in one of the EntryListCar
	DriverId       uint16  // index in the EntryListCar.Drivers array to indicate current driver
	DriverCount    uint8   // total count of drivers, thus be same as number of drivers declared in EntryListCar
	Gear           int8    // 0 is neurtral
	WorldPosX      float32 // always == 0
	WorldPosY      float32 // always == 0
	Yaw            float32 // always == 0
	CarLocation    uint8   // See const declarations CarLocation<name>
	Kmh            uint16  // self-explanatory
	Position       uint16  // not sure yet when updated
	CupPosition    uint16  // not sure yet when updated
	TrackPosition  uint16  // always == 0
	SplinePosition float32 // between 0 and 1 indicating where the car is on track, not sure yet what when car is in pit
	Laps           uint16  // number of laps completed. Thus zero during first lap of the race. Note: also 0 before the start of the race
	Delta          int32   // delta in respect to its fastest lap in ms
	BestSessionLap Lap
	LastLap        Lap

	// The LapTimeMs is continuously updated during the lap.
	// The splits of the CurrentLap are however never filled in.
	CurrentLap Lap
}

type BroadCastEvent struct {
	Type   byte   // ?
	Msg    string // ?
	TimeMs int32  // !SessionTime is a float however (int32 is better than float though)
	CarId  int32  // !elsewhere this is uint16
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
	FirstName string
	LastName  string
	ShortName string
	Category  byte
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
	ok = readBuffer(buffer, &car.Id)
	ok = ok && readBuffer(buffer, &car.Model)
	ok = ok && readString(buffer, &car.TeamName)
	ok = ok && readBuffer(buffer, &car.RaceNumber)
	ok = ok && readBuffer(buffer, &car.CupCategory)
	ok = ok && readBuffer(buffer, &car.CurrentDriverId)

	var driversOnCarCount uint8
	ok = ok && readBuffer(buffer, &driversOnCarCount)
	car.Drivers = make([]Driver, driversOnCarCount)
	for i := uint8(0); ok && i < driversOnCarCount; i++ {
		ok = ok && readString(buffer, &car.Drivers[i].FirstName)
		ok = ok && readString(buffer, &car.Drivers[i].LastName)
		ok = ok && readString(buffer, &car.Drivers[i].ShortName)
		ok = ok && readBuffer(buffer, &(car.Drivers[i].Category))
	}
	return car, ok
}

func MarshalTrackDataReq(buffer *bytes.Buffer, connectionId int32) bool {
	ok := writeByteBuffer(buffer, RequestTrackData)
	ok = ok && writeBuffer(buffer, connectionId)
	return ok
}

func UnmarshalTrackDataResp(buffer *bytes.Buffer) (connectionId int32, trackData TrackData, ok bool) {
	ok = readBuffer(buffer, &connectionId)
	ok = readString(buffer, &trackData.Name)
	ok = ok && readBuffer(buffer, &trackData.Id)
	ok = ok && readBuffer(buffer, &trackData.Meters)
	return connectionId, trackData, ok
}

func unmarshalRealTimeUpdate(buffer *bytes.Buffer) (update RealTimeUpdate, ok bool) {
	ok = readBuffer(buffer, &update.EventIndex)
	ok = ok && readBuffer(buffer, &update.SessionIndex)
	ok = ok && readBuffer(buffer, &update.SessionType)
	ok = ok && readBuffer(buffer, &update.Phase)
	ok = ok && readBuffer(buffer, &update.SessionTime)
	ok = ok && readBuffer(buffer, &update.SessionEndTime)
	ok = ok && readBuffer(buffer, &update.FocusedCarIndex)
	ok = ok && readString(buffer, &update.ActiveCameraSet)
	ok = ok && readString(buffer, &update.ActiveCamera)
	ok = ok && readBuffer(buffer, &update.IsReplayPlaying)
	ok = ok && readBuffer(buffer, &update.TimeOfDay)
	ok = ok && readBuffer(buffer, &update.AmbientTemp)
	ok = ok && readBuffer(buffer, &update.TrackTemp)
	ok = ok && readBuffer(buffer, &update.Clouds)
	ok = ok && readBuffer(buffer, &update.RainLevel)
	ok = ok && readBuffer(buffer, &update.Wettness)
	if ok {
		update.BestSessionLap, ok = unmarshalLap(buffer)
	}

	return update, ok
}

func UnmarshalCarUpdateResp(buffer *bytes.Buffer) (carUpdate RealTimeCarUpdate, ok bool) {
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
		carUpdate.BestSessionLap, ok = unmarshalLap(buffer)
	}
	if ok {
		carUpdate.LastLap, ok = unmarshalLap(buffer)
	}
	if ok {
		carUpdate.CurrentLap, ok = unmarshalLap(buffer)
	}
	return carUpdate, ok
}

func unmarshalBroadCastEvent(buffer *bytes.Buffer) (broadCastEvent BroadCastEvent, ok bool) {
	ok = readBuffer(buffer, &broadCastEvent.Type)
	ok = ok && readString(buffer, &broadCastEvent.Msg)
	ok = ok && readBuffer(buffer, &broadCastEvent.TimeMs)
	ok = ok && readBuffer(buffer, &broadCastEvent.CarId)
	return broadCastEvent, ok
}

func unmarshalLap(buffer *bytes.Buffer) (lap Lap, ok bool) {
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
