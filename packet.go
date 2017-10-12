package battleye

import (
	"encoding/binary"
	"hash/crc32"
)

// packet represents a BattlEye packet.
type packet struct {
	message        string
	payloadType    payloadType
	sequenceNumber byte
}

// newLoginPacket returns a new loginType packet with the password set.
func newLoginPacket(password string) *packet {
	return &packet{payloadType: loginType, message: password}
}

// newCommandPacket returns a new commandType packet with the message and sequence number set.
func newCommandPacket(command string, sequence byte) *packet {
	return &packet{payloadType: commandType, message: command, sequenceNumber: sequence}
}

// newServerMessageAcknowledgePacket returns a new serverMessageType packet with the sequence number set.
func newServerMessageAcknowledgePacket(sequence byte) *packet {
	return &packet{payloadType: serverMessageType, sequenceNumber: sequence}
}

// bytes returns the packet as []byte.
func (p *packet) bytes() ([]byte, error) {
	payload, err := p.payload()
	if err != nil {
		return nil, err
	}
	header := p.header(payload)
	return append(header, payload...), nil
}

// header returns the packet header as []byte.
func (p *packet) header(payload []byte) []byte {
	data := []byte{0x42, 0x45, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(data[2:6], crc32.ChecksumIEEE(payload))
	return data
}

// payload returns the packet payload as []byte.
func (p *packet) payload() ([]byte, error) {
	switch p.payloadType {
	case loginType:
		return append([]byte{0xff, byte(p.payloadType)}, []byte(p.message)...), nil
	case commandType:
		return append([]byte{0xff, byte(p.payloadType)}, append([]byte{p.sequenceNumber}, []byte(p.message)...)...), nil
	case serverMessageType:
		return append([]byte{0xff, byte(p.payloadType)}, p.sequenceNumber), nil
	default:
		return nil, ErrUnknownPacketType
	}
}
