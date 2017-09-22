package battleye

import (
	"errors"
)

var (
	// ErrInvalidMessageBufferSize is returned if MessageBuffer Option is used with a size less than 1.
	ErrInvalidMessageBufferSize = errors.New("battleye: invalid message buffer size")

	// ErrInvalidPacketSize is returned if the packet size is less than the minimum size.
	ErrInvalidPacketSize = errors.New("battleye: invalid packet size")

	// ErrInvalidHeader is returned if packet does not start with 0x42, 0x45 (BE).
	ErrInvalidHeader = errors.New("battleye: invalid header")

	// ErrInvalidChecksum is returned the checksum in the packet header is invalid.
	ErrInvalidChecksum = errors.New("battleye: invalid checksum")

	// ErrInvalidEndOfHeader is returned if the last byte of the header is not 0xff.
	ErrInvalidEndOfHeader = errors.New("battleye: invalid end of header")

	// ErrUnknownPacketType is returned if packet type cannot be determined.
	ErrUnknownPacketType = errors.New("battleye: unknown packet type")

	// ErrInvalidLoginResponse is returned if the response byte in the login response is invalid.
	ErrInvalidLoginResponse = errors.New("battleye: invalid login response")

	// ErrNilOption is returned by NewClient if an Option is nil.
	ErrNilOption = errors.New("battleye: nil option")

	// ErrLoginFailed is returned by NewClient if it was unable to connect to the server due to auth failure.
	ErrLoginFailed = errors.New("battleye: login failed")

	// ErrTimeout is returned after the timeout period elapsed while waiting for response or error from the BattlEye server.
	ErrTimeout = errors.New("battleye: timeout")
)
