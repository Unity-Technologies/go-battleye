package battleye

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testPassword = "secret"
)

// nolint: gocyclo
func TestClient(t *testing.T) {
	testcases := []struct {
		name           string
		clientPassword string
		clientOpts     []Option
		keepAliveCheck time.Duration
		expClientErr   error
		closesClient   bool
		testfunc       func(*testing.T, *Client, *server)
	}{
		{
			name:         "Invalid MessageBuffer option",
			clientOpts:   []Option{MessageBuffer(0)},
			expClientErr: ErrInvalidMessageBufferSize,
		},
		{
			name:           "Failed login",
			clientPassword: "not the right password",
			expClientErr:   ErrLoginFailed,
			clientOpts:     []Option{Timeout(testTimeout)},
		},
		{
			name:       "Closing a closed Client doesn't panic",
			clientOpts: []Option{Timeout(1 * time.Second)},
			testfunc: func(t *testing.T, c *Client, s *server) {
				c.Close()                                 // nolint: errcheck
				assert.NotPanics(t, func() { c.Close() }) // nolint: errcheck
			},
		},
		{
			name:       "Multi-packet response",
			clientOpts: []Option{Timeout(1 * time.Second)},
			testfunc: func(t *testing.T, c *Client, s *server) {
				s.SetMultiPacketResponse("part 1* part 2* part 3* part 4* part 5* part 6")

				resp, err := c.Exec("status")
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, "part 1 part 2 part 3 part 4 part 5 part 6", resp)
			},
		},
		{
			name:           "Successful login, keep-alive and execute command",
			clientOpts:     []Option{Timeout(testTimeout), KeepAlive(200 * time.Millisecond), MessageBuffer(10)},
			closesClient:   true,
			keepAliveCheck: time.Millisecond * 10,
			testfunc: func(t *testing.T, c *Client, s *server) {
				defer func() {
					if !assert.NoError(t, c.Close()) {
						return
					}

					// mock server must have received some keep alive packets
					v := s.keepAlive()
					assert.True(t, v >= 1, fmt.Sprintf("keep-alive: %v not greater than one", v))
					// mock server must have received some server message acknowledge packets
					assert.True(t, s.srvMsgAck() >= 1)
					// we must have received at least 1 server message
					assert.True(t, len(c.Messages()) >= 1)
				}()

				// wait for keep alive and server message packets
				time.Sleep(1 * time.Second)

				// test command response
				resp, err := c.Exec("status")
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, "Response to: status", resp)
			},
		},
		{
			name:       "Only one response is expected to our message",
			clientOpts: []Option{Timeout(1 * time.Second)},
			testfunc: func(t *testing.T, c *Client, s *server) {
				s.SetDuplicatedResponse()

				resp, err := c.Exec("status")
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, "Response to: status", resp)

				resp, err = c.Exec("players")
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, "Response to: players", resp)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := newServer(t, testPassword)
			if s == nil {
				return
			}
			s.Start()
			defer s.Close()

			if tc.clientPassword == "" {
				tc.clientPassword = testPassword
			}

			if tc.keepAliveCheck > 0 {
				old := keepAliveCheck
				keepAliveCheck = tc.keepAliveCheck
				defer func() { keepAliveCheck = old }()
			}

			c, err := NewClient(s.Addr, tc.clientPassword, tc.clientOpts...)
			defer func() {
				if !tc.closesClient && c != nil {
					assert.NoError(t, c.Close())
				}
			}()
			if tc.expClientErr != nil {
				assert.Nil(t, c)
				assert.EqualError(t, err, tc.expClientErr.Error())
				return
			}
			if !assert.NoError(t, err) {
				return
			}
			if tc.testfunc == nil {
				return
			}
			tc.testfunc(t, c, s)
		})
	}
}
