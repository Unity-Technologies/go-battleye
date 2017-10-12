// +build integration

package battleye

import (
	"flag"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	address  = flag.String("address", "127.0.0.1:2301", "sets the server address for integration tests")
	password = flag.String("password", "", "sets the server password for integration tests")
	timeout  = flag.Duration("client-timeout", 2*time.Second, "sets the read/write timeout of the client")

	versionRegexp = regexp.MustCompile(`\d\.\d{3}`)
)

func TestIntegration(t *testing.T) {
	c, err := NewClient(*address, *password, Timeout(*timeout))
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, c.Close())
	}()

	assertCommand(t, c, "players", func(resp string) bool {
		return strings.Contains(resp, "Players on server:")
	})
	assertCommand(t, c, "version", func(resp string) bool {
		return versionRegexp.MatchString(resp)
	})
}

func assertCommand(t *testing.T, c *Client, cmd string, f func(resp string) bool) {
	resp, err := c.Exec(cmd)
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, f(resp))
}
