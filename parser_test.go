package battleye

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const unexpectedSequenceNumber = "unexpected sequence number"

// nolint: gocyclo
func TestResponseParser(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name        string
		raw         []byte
		expErr      error
		expResponse func(interface{}) (bool, string)
	}{
		{
			name:        "Invalid packet size",
			raw:         []byte{0, 0, 0, 0, 0, 0, 0, 0},
			expErr:      ErrInvalidPacketSize,
			expResponse: nil,
		},
		{
			name:        "Invalid packet ID",
			raw:         []byte{0x47, 0x47, 0, 0, 0, 0, 0, 0, 0},
			expErr:      ErrInvalidHeader,
			expResponse: nil,
		},
		{
			name:        "Invalid end of header",
			raw:         []byte{0x42, 0x45, 0x12, 0xd9, 0x41, 0xff, 0, 0, 0},
			expErr:      ErrInvalidEndOfHeader,
			expResponse: nil,
		},
		{
			name:        "Invalid checksum",
			raw:         []byte{0x42, 0x45, 0, 0x23, 0, 0x85, 0xff, 0, 0},
			expErr:      ErrInvalidChecksum,
			expResponse: nil,
		},
		{
			name:        "Unknown packet type",
			raw:         []byte{0x42, 0x45, 0xba, 0x19, 0xae, 0x3c, 0xff, 0x05, 0},
			expErr:      ErrUnknownPacketType,
			expResponse: nil,
		},
		{
			name:   "loginResponse with successful login",
			raw:    []byte{0x42, 0x45, 0x69, 0xdd, 0xde, 0x36, 0xff, 0x00, 0x01},
			expErr: nil,
			expResponse: func(r interface{}) (bool, string) {
				switch tp := r.(type) {
				case bool:
					return assert.True(t, tp), "loginResponse should be: loginSuccess"
				default:
					return false, fmt.Sprintf("not loginType: %v", tp)
				}
			},
		},
		{
			name:   "loginResponse with failed login",
			raw:    []byte{0x42, 0x45, 0xff, 0xed, 0xd9, 0x41, 0xff, 0x00, 0x00},
			expErr: nil,
			expResponse: func(r interface{}) (bool, string) {
				switch tp := r.(type) {
				case bool:
					return assert.False(t, tp), "loginResponse should be: loginFailed"
				default:
					return false, fmt.Sprintf("not loginType: %v", tp)
				}
			},
		},
		{
			name:        "loginResponse with invalid response",
			raw:         []byte{0x42, 0x45, 0xd3, 0x8c, 0xd7, 0xaf, 0xff, 0x00, 0x02},
			expErr:      ErrInvalidLoginResponse,
			expResponse: nil,
		},
		{
			name:   "commandResponse",
			raw:    append([]byte{0x42, 0x45, 0x01, 0x7f, 0xb1, 0x1f, 0xff, 0x01, 0x01}, []byte("Hello")...),
			expErr: nil,
			expResponse: func(r interface{}) (bool, string) {
				switch tp := r.(type) {
				case *commandResponse:
					assert.Equal(t, "Hello", tp.msg)
					return assert.Equal(t, byte(1), tp.seq), unexpectedSequenceNumber
				default:
					return false, fmt.Sprintf("not commandType: %v", tp)
				}
			},
		},
		{
			name:   "commandResponse empty response",
			raw:    []byte{0x42, 0x45, 0x04, 0x8d, 0xcb, 0xc1, 0xff, 0x01, 0x03},
			expErr: nil,
			expResponse: func(r interface{}) (bool, string) {
				switch tp := r.(type) {
				case *commandResponse:
					assert.Equal(t, "", tp.msg)
					return assert.Equal(t, byte(3), tp.seq), unexpectedSequenceNumber
				default:
					return false, fmt.Sprintf("not commandType: %v", tp)
				}
			},
		},
		{
			name:   "serverMessage",
			raw:    append([]byte{0x42, 0x45, 0x6a, 0x44, 0x98, 0x60, 0xff, 0x02, 0x07}, []byte("Server message")...),
			expErr: nil,
			expResponse: func(r interface{}) (bool, string) {
				switch tp := r.(type) {
				case *serverMessage:
					assert.Equal(t, "Server message", tp.msg)
					return assert.Equal(t, byte(7), tp.seq), unexpectedSequenceNumber
				default:
					return false, fmt.Sprintf("not serverMessage: %v", tp)
				}
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(*testing.T) {
			resp, err := parseResponse(tc.raw)

			if tc.expErr != nil {
				assert.EqualError(t, err, tc.expErr.Error())
			} else {
				if !assert.NoError(t, err) {
					return
				}
				v, errMsg := tc.expResponse(resp)
				assert.True(t, v, errMsg)
			}
		})
	}
}
