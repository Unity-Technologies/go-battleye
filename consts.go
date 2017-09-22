package battleye

// payloadType specifies the message type of the payload.
type payloadType byte

// BattlEye payload types.
const (
	loginType payloadType = iota
	commandType
	serverMessageType

	// multiPacketType is an optional embedded header type inside a commandType payload.
	multiPacketType byte = 0
)

// loginResult specified the result of a login attempt.
type loginResult byte

// loginResponse messages.
const (
	loginFailed loginResult = iota
	loginSuccess
)
