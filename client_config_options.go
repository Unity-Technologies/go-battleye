package battleye

import (
	"time"
)

// Option is a Client configuration Option type.
type Option func(c *Client) error

// Timeout sets the connection read / write timeout for a Client.
func Timeout(timeout time.Duration) Option {
	return func(c *Client) error {
		c.timeout = timeout
		return nil
	}
}

// KeepAlive sets the keep-alive message interval for a Client.
func KeepAlive(interval time.Duration) Option {
	return func(c *Client) error {
		c.keepAlive = interval
		return nil
	}
}

// MessageBuffer sets the size of the Messages channel buffer.
func MessageBuffer(size int) Option {
	return func(c *Client) error {
		if size < 1 {
			return ErrInvalidMessageBufferSize
		}
		c.msgBufSize = size
		return nil
	}
}
