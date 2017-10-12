package battleye

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// defaultTimeout is the default read / write timeout for the Client.
	defaultTimeout = 10 * time.Second

	// defaultKeepAlive is the default interval of sending keep-alive packets to the BattlEye server.
	// It must be less than 45 seconds because BattlEye server consider a client disconnected
	// if no command packets are received from it for more than 45 seconds.
	defaultKeepAlive = 30 * time.Second

	// defaultMessageBufferSize is the default buffer size of the msgs channel.
	defaultMessageBufferSize = 100

	// bufferSize is the size of the read buffer based on MTU.
	bufferSize = 1500

	// clientTimeout is the maximum duration after which the Client will be disconnected.
	clientTimeout = 45 * time.Second
)

var (
	// keepAliveCheck is the check interval for keepalives
	keepAliveCheck = time.Second
)

// Client represents a BattlEye client.
type Client struct {
	conn       net.Conn
	ctr        uint64
	timeout    time.Duration
	keepAlive  time.Duration
	msgBufSize int
	wg         sync.WaitGroup
	fragments  map[byte]*fragmentedResponse
	sendLock   sync.Mutex
	lastLock   sync.Mutex
	lastSend   time.Time

	// done signals goroutines to stop.
	done *done

	// login is used for receiving the login response from the BattlEye server.
	login chan bool

	// cmds is used for receiving command-type responses from the BattlEye server.
	cmds chan string

	// msgs is a buffered channel which is used for getting broadcast messages from the BattlEye server.
	msgs chan string

	// errs is a channel for transmitting errors internally.
	errs chan error
}

// NewClient returns a new BattlEye client connected to address.
func NewClient(addr string, pwd string, options ...Option) (*Client, error) {
	c := &Client{
		timeout:    defaultTimeout,
		keepAlive:  defaultKeepAlive,
		msgBufSize: defaultMessageBufferSize,
	}

	// Override defaults
	for _, opt := range options {
		if opt == nil {
			return nil, ErrNilOption
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	c.done = newDone()
	c.login = make(chan bool)
	c.cmds = make(chan string)
	c.msgs = make(chan string, c.msgBufSize)
	c.errs = make(chan error)

	c.fragments = make(map[byte]*fragmentedResponse)

	if err := c.connect(addr, pwd); err != nil {
		c.Close() // nolint: errcheck
		return nil, err
	}

	return c, nil
}

// Close gracefully closes the connection.
func (c *Client) Close() error {
	if c.done.IsDone() {
		return nil
	}
	c.done.Done()
	c.wg.Wait()
	close(c.msgs)
	return c.conn.Close()
}

// Messages returns a buffered channel containing the console messages sent by the server.
// If the channel is full new messages will be dropped. It is the user's responsibility
// to drain the channel and handle these messages.
func (c *Client) Messages() <-chan string {
	return c.msgs
}

// Exec executes the cmd on the BattlEye server and returns its response.
// Executing is retried for 45 seconds after which the Client is considered to be disconnected
// and ErrTimeout is returned.
// A disconnected Client is unlikely to get any more responses from the BattlEye server, so
// a new Client should be created.
func (c *Client) Exec(cmd string) (string, error) {
	c.sendLock.Lock()
	defer c.sendLock.Unlock()

	until := time.Now().Add(clientTimeout)
	for time.Now().Before(until) {
		resp, err := c.send(cmd)
		if err != nil {
			if err == ErrTimeout {
				continue
			}
			return "", err
		}
		return resp, nil
	}

	// TODO: Is this fatal? Do we close the Client at this point?
	return "", ErrTimeout
}

func (c *Client) send(cmd string) (string, error) {
	if err := c.write(newCommandPacket(cmd, c.seq())); err != nil {
		return "", err
	}

	c.lastLock.Lock()
	c.lastSend = time.Now()
	c.lastLock.Unlock()

	t := time.NewTimer(c.timeout)
	defer t.Stop()
	select {
	case <-t.C:
		return "", ErrTimeout
	case err := <-c.errs:
		return "", err
	case resp := <-c.cmds:
		return resp, nil
	}
}

// connect connects and authenticates Client to the BattlEye server.
func (c *Client) connect(addr, pwd string) (err error) {
	c.conn, err = net.Dial("udp", addr)
	if err != nil {
		return err
	}

	c.wg.Add(1)
	go c.receiver()

	// Authenticate client.
	if err := c.write(newLoginPacket(pwd)); err != nil {
		return err
	}

	t := time.NewTimer(c.timeout)
	select {
	case <-t.C:
		return ErrTimeout
	case err := <-c.errs:
		return err
	case success := <-c.login:
		if !success {
			return ErrLoginFailed
		}
	}

	// Client successfully logged in, start the keep-alive goroutine.
	c.wg.Add(1)
	go c.keepConnectionAlive()

	return nil
}

// keepConnectionAlive is a goroutine which periodically sends a keep-alive packet to the BattlEye server.
func (c *Client) keepConnectionAlive() {
	defer c.wg.Done()

	t := time.NewTicker(keepAliveCheck)
	for {
		select {
		case <-c.done.C():
			t.Stop()
			return
		case <-t.C:
			// To avoid potential deadlocks, this check should be done separately.
			c.lastLock.Lock()
			do := time.Since(c.lastSend) > c.keepAlive
			c.lastLock.Unlock()

			if do {
				// Send an empty command, we don't care the response nor the error.
				c.Exec("") // nolint: errcheck
			}
		}
	}
}

// write writes a packet to conn.
func (c *Client) write(pkt *packet) error {
	raw, err := pkt.bytes()
	if err != nil {
		return err
	}
	if err = c.setDeadline(); err != nil {
		return err
	}
	_, err = c.conn.Write(raw)
	return err
}

// receiver is a goroutine which reads responses from the connection and handles them according to
// their types.
func (c *Client) receiver() {
	defer c.wg.Done()

	for {
		select {
		case <-c.done.C():
			return
		default:
			r, err := c.read()
			if err != nil {
				// Do not error in case of timeout.
				if err, ok := err.(net.Error); ok && err.Timeout() {
					continue
				}
				c.errs <- err
				continue
			}
			switch r := r.(type) {
			case bool:
				c.login <- r
			case *commandResponse:
				c.handleCommandResponse(r)
			case *serverMessage:
				c.handleServerMessage(r)
			}
		}
	}
}

// handleCommandResponse forwards CommandResponses to the cmds channel. If the message is
// fragmented it is reassembled beforehand.
func (c *Client) handleCommandResponse(r *commandResponse) {
	// If the received response is either:
	// - an old one that we've already processed (sequence number is less than what we expect);
	// - or an unsolicited one (sequence number it totally different from what we expect);
	// just drop it.
	if r.seq != c.seq() {
		return
	}

	// response is not fragmented.
	if !r.multi {
		c.incr()
		c.cmds <- r.msg
		return
	}

	// Add the partial message to the already received parts.
	var fr *fragmentedResponse
	fr, ok := c.fragments[r.seq]
	if !ok {
		fr = newFragmentedResponse(r.multiSize)
		c.fragments[r.seq] = fr
	}
	fr.add(r)

	// If the message is complete send it.
	if fr.completed() {
		c.incr()
		c.cmds <- fr.message()
	}
}

// handleServerMessage forwards the message part of ServerMessages to the msgs channel and
// sends back an acknowledge packet to the server.
func (c *Client) handleServerMessage(r *serverMessage) {
	// If the channel is full, new messages will be dropped.
	select {
	case c.msgs <- r.msg:
	default:
	}
	// Client has to acknowledge the server message by sending back its sequence number.
	// No response is expected from the server.
	// We don't care write errors.
	c.write(newServerMessageAcknowledgePacket(r.seq)) // nolint: errcheck
}

// read reads from conn and parses the raw data as response.
func (c *Client) read() (interface{}, error) {
	if err := c.setDeadline(); err != nil {
		return nil, err
	}
	// As there is no size in the battleye protocol we must assume each read returns a single response.
	b := make([]byte, bufferSize)
	n, err := c.conn.Read(b)
	if err != nil {
		return nil, err
	}
	return parseResponse(b[:n])
}

// seq returns the command sequence number counter.
func (c *Client) seq() byte {
	return byte(atomic.LoadUint64(&c.ctr))
}

// incr increments the command sequence number counter.
func (c *Client) incr() {
	atomic.AddUint64(&c.ctr, 1)
}

// setDeadline updates the deadline on the connection based on the clients configured timeout.
func (c *Client) setDeadline() error {
	return c.conn.SetDeadline(time.Now().Add(c.timeout))
}
