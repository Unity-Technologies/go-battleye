package battleye

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"strings"
)

const (
	// minPacketSize is the smallest possible packet size in bytes.
	minPacketSize = 9
)

// parseResponse parses raw data and returns a new response if successful.
func parseResponse(raw []byte) (interface{}, error) {
	if len(raw) < minPacketSize {
		return nil, ErrInvalidPacketSize
	}

	if !bytes.Equal(raw[0:2], []byte{0x42, 0x45}) {
		return nil, ErrInvalidHeader
	}

	if raw[6] != 0xff {
		return nil, ErrInvalidEndOfHeader
	}

	if crc32.ChecksumIEEE(raw[6:]) != binary.LittleEndian.Uint32(raw[2:6]) {
		return nil, ErrInvalidChecksum
	}

	switch payloadType(raw[7]) {
	case loginType:
		return loginResponse(raw[8:])
	case commandType:
		return newCommandResponse(raw[8:])
	case serverMessageType:
		return newServerMessage(raw[8:])
	default:
		return nil, ErrUnknownPacketType
	}
}

// loginResponse parses raw bytes and returns a new loginResponse if successful.
func loginResponse(raw []byte) (bool, error) {
	switch loginResult(raw[0]) {
	case loginFailed:
		return false, nil
	case loginSuccess:
		return true, nil
	default:
		return false, ErrInvalidLoginResponse
	}
}

// commandResponse is the response type that BattlEye server replies with to incoming command packets.
type commandResponse struct {
	msg        string
	seq        byte
	multiSize  byte
	multiIndex byte
	multi      bool
}

// newCommandResponse parses raw bytes and returns a new commandResponse if successful.
func newCommandResponse(raw []byte) (*commandResponse, error) {
	cr := &commandResponse{seq: raw[0]}
	if len(raw[1:]) == 0 {
		return cr, nil
	}

	restIndex := 1
	if raw[1] == multiPacketType {
		cr.multi = true
		cr.multiSize, cr.multiIndex = raw[2], raw[3]
		restIndex = 4
	}
	cr.msg = string(raw[restIndex:])

	return cr, nil
}

// serverMessage is the type of packet that BattlEye server sends to clients.
type serverMessage struct {
	seq byte
	msg string
}

// newServerMessage parses raw bytes and returns a new serverMessage if successful.
func newServerMessage(raw []byte) (*serverMessage, error) {
	return &serverMessage{seq: raw[0], msg: string(raw[1:])}, nil
}

// fragmentedResponse represents a commandResponse sent in multiple packets.
type fragmentedResponse struct {
	expected map[byte]struct{}
	parts    []string
}

// newFragmentedResponse returns a new fragmentedMessage initialized to handle a number of
// message parts equal to size.
func newFragmentedResponse(size byte) *fragmentedResponse {
	m := make(map[byte]struct{})
	for i := byte(0); i < size; i++ {
		m[i] = struct{}{}
	}
	return &fragmentedResponse{
		expected: m,
		parts:    make([]string, size),
	}
}

// add stores the partial message and original part index from cr.
func (fm *fragmentedResponse) add(cr *commandResponse) {
	fm.parts[cr.multiIndex] = cr.msg
	delete(fm.expected, cr.multiIndex)
}

// completed returns true if all parts have been added.
func (fm *fragmentedResponse) completed() bool {
	return len(fm.expected) == 0
}

// message returns the parts joined in the original order.
func (fm *fragmentedResponse) message() string {
	return strings.Join(fm.parts, "")
}
