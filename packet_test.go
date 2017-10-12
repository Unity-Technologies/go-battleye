package battleye

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPacket(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name              string
		packet            *packet
		expType           payloadType
		expMessage        string
		expSequenceNumber byte
	}{
		{
			name:       "Login packet",
			packet:     newLoginPacket("secret"),
			expType:    loginType,
			expMessage: "secret",
		},
		{
			name:              "Server message acknowledge packet",
			packet:            newServerMessageAcknowledgePacket(2),
			expType:           serverMessageType,
			expSequenceNumber: 2,
		},
		{
			name:              "Command packet",
			packet:            newCommandPacket("do something", 3),
			expType:           commandType,
			expSequenceNumber: 3,
			expMessage:        "do something",
		},
		{
			name:              "Keep-alive packet",
			packet:            newCommandPacket("", 4),
			expType:           commandType,
			expSequenceNumber: 4,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tc.packet.bytes()
			if !assert.NoError(t, err) {
				return
			}
			if !assert.True(t, int64(len(p)) >= minPacketSize) {
				return
			}

			assert.Equal(t, []byte("BE"), p[:2])
			assert.Equal(t, binary.LittleEndian.Uint32(p[2:6]), crc32.ChecksumIEEE(p[6:]))
			assert.Equal(t, byte(0xff), p[6])

			if !assert.Equal(t, byte(tc.expType), p[7]) {
				return
			}
			switch tc.expType {
			case loginType:
				assert.Equal(t, []byte(tc.expMessage), p[8:])
			case commandType:
				assert.Equal(t, byte(tc.expSequenceNumber), p[8])
				assert.Equal(t, []byte(tc.expMessage), p[9:])
			case serverMessageType:
				assert.Equal(t, byte(tc.expSequenceNumber), p[8])
			}
		})
	}
}

func TestUnknownPacket(t *testing.T) {
	p := &packet{payloadType: 5}
	_, err := p.bytes()
	assert.EqualError(t, err, ErrUnknownPacketType.Error())
}
