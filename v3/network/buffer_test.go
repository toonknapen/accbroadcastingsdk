package network

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// In the ACC interface, all data is stored little-endian
func TestInt16LittleBigEndian(t *testing.T) {
	sixteenBits := []byte{0x01, 0x00, 0x01, 0x00}
	sixteenBitsBuffer := bytes.NewBuffer(sixteenBits)
	var intSixteen int16
	err := binary.Read(sixteenBitsBuffer, binary.LittleEndian, &intSixteen)
	if err != nil || intSixteen != 1 {
		t.Fail()
	}
	err = binary.Read(sixteenBitsBuffer, binary.BigEndian, &intSixteen)
	if err != nil || intSixteen != 256 {
		t.Fail()
	}
}

// just to show the short-circuit trick used to stop (un)marshaling from the moment an error is encountered
func TestShortCircuitAnd(t *testing.T) {
	isCalled := false
	ok := false
	ok = ok && isCalledFn(&isCalled)
	if isCalled != false {
		t.Fail()
	}
}

func TestShortCircuitOr(t *testing.T) {
	isCalled := false
	ok := true
	ok = ok && isCalledFn(&isCalled)
	if isCalled != true {
		t.Fail()
	}
}

func isCalledFn(isCalled *bool) bool {
	*isCalled = true
	return true
}
