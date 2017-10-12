package battleye

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testTimeout       = time.Second
	testAddress       = "127.0.0.1:0"
	testServerMessage = "server broadcast"
)

type multiResponse struct {
	Index   int
	Message string
}

// server is a mock source rcon server
type server struct {
	Addr string
	pwd  string
	pc   net.PacketConn
	t    *testing.T
	done chan struct{}
	wg   sync.WaitGroup

	keepAliveCounter int64
	srvMsgAckCounter int64
	clients          sync.Map
	seq              byte
	multiRespCh      chan string
	duplicateCh      chan struct{}
}

// newServer returns a server or nil if an error occurred.
func newServer(t *testing.T, pwd string) *server {
	pc, err := net.ListenPacket("udp", testAddress)
	if !assert.NoError(t, err) {
		return nil
	}

	s := &server{
		Addr: pc.LocalAddr().String(),
		pwd:  pwd,
		done: make(chan struct{}),
		t:    t,
		pc:   pc,

		multiRespCh: make(chan string, 1),
		duplicateCh: make(chan struct{}, 1),
	}

	return s
}

// Close cleanly shuts down the server.
func (s *server) Close() {
	close(s.done)
	s.wg.Wait()
	s.pc.Close() // nolint: errcheck
}

// Start starts the server.
func (s *server) Start() {
	s.wg.Add(2)
	go s.serve()
	go s.messenger()
}

// SetMultiPacketResponse sets a command response message to be sent back fragmented in multiple packets.
// The message is split by every * character.
func (s *server) SetMultiPacketResponse(message string) {
	s.multiRespCh <- message
}

func (s *server) SetDuplicatedResponse() {
	s.duplicateCh <- struct{}{}
}

// keepAlive returns how many keep-alive messages the server received.
func (s *server) keepAlive() int {
	return int(atomic.LoadInt64(&s.keepAliveCounter))
}

// incrKeepAlive increments the keep-alive message counter.
func (s *server) incrKeepAlive() {
	atomic.AddInt64(&s.keepAliveCounter, 1)
}

// srvMsgAck returns how many server message ack messages the server received.
func (s *server) srvMsgAck() int {
	return int(atomic.LoadInt64(&s.srvMsgAckCounter))
}

// incrSrvMsgAck increments the server message ack counter.
func (s *server) incrSrvMsgAck() {
	atomic.AddInt64(&s.srvMsgAckCounter, 1)
}

// serve listens to incoming messages and responds to them.
func (s *server) serve() {
	defer s.wg.Done()

	for {
		select {
		case <-s.done:
			return
		default:
			buffer := make([]byte, bufferSize)
			if err := s.pc.SetDeadline(time.Now().Add(testTimeout)); err != nil {
				return
			}
			n, addr, err := s.pc.ReadFrom(buffer)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					continue
				}
				assert.Fail(s.t, err.Error())
				return
			}
			s.clients.Store(addr.String(), addr)
			if err := s.handleMessage(buffer[:n], addr); err != nil {
				assert.Fail(s.t, err.Error())
				return
			}
		}
	}
}

// messenger sends a server message to each known client regularly.
func (s *server) messenger() {
	defer s.wg.Done()

	t := time.NewTicker(time.Millisecond * 100)
	defer t.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-t.C:
			p := createServerMessage(s.seq)
			s.clients.Range(func(k, v interface{}) bool {
				if addr, ok := v.(net.Addr); ok {
					_, err := s.pc.WriteTo(p, addr)
					return assert.NoError(s.t, err)
				}
				return false
			})
			s.seq++
		}
	}
}

// handleMessage determines the type of the message and handles them accordingly.
func (s *server) handleMessage(b []byte, addr net.Addr) error {
	if int64(len(b)) < minPacketSize {
		return ErrInvalidPacketSize
	}
	switch payloadType(b[7]) {
	case loginType:
		return s.handleLoginMessage(b, addr)
	case commandType:
		return s.handleCommandMessage(b, addr)
	case serverMessageType:
		// Server does not have to reply to ack messages.
		s.incrSrvMsgAck()
		return nil
	default:
		return ErrUnknownPacketType
	}
}

// handleLoginMessage checks the password in the message and sends a login success/failed message accordingly.
func (s *server) handleLoginMessage(b []byte, addr net.Addr) error {
	p := &packet{payloadType: loginType, message: string(0x00)}
	pwd := string(b[8:])
	if pwd == s.pwd {
		p.message = string(0x01)
	}
	return s.sendPacket(p, addr)
}

// handleCommandMessage responds to command type messages.
func (s *server) handleCommandMessage(b []byte, addr net.Addr) error {
	seq := b[8]

	if int64(len(b)) == minPacketSize {
		s.incrKeepAlive()
		p := &packet{payloadType: commandType, sequenceNumber: seq}
		return s.sendPacket(p, addr)
	}

	select {
	case msg := <-s.multiRespCh:
		parts := strings.Split(msg, "*")
		messages := make([]multiResponse, len(parts))
		for i := range parts {
			messages[i] = multiResponse{Index: i, Message: parts[i]}
		}
		// send the packets in random order to simulate network latency
		for _, i := range rand.Perm(len(parts)) {
			pb := createMultiResponse(messages[i].Message, seq, byte(len(parts)), byte(messages[i].Index))
			if err := s.sendBytes(pb, addr); err != nil {
				return err
			}
		}
		return nil
	default:
		p := &packet{payloadType: commandType, sequenceNumber: seq, message: "Response to: " + string(b[9:])}
		if err := s.sendPacket(p, addr); err != nil {
			return err
		}
		select {
		case <-s.duplicateCh:
			p.message += " (duplicate)"
			return s.sendPacket(p, addr)
		default:
			return nil
		}
	}
}

// sendPacket sends p to addr.
func (s *server) sendPacket(p *packet, addr net.Addr) error {
	b, err := p.bytes()
	if err != nil {
		return err
	}
	return s.sendBytes(b, addr)
}

// sendBytes sends b to addr.
func (s *server) sendBytes(b []byte, addr net.Addr) error {
	_, err := s.pc.WriteTo(b, addr)
	return err
}

// createMultiResponse creates a multi command response packet.
func createMultiResponse(message string, seq, max, current byte) []byte {
	multiHeader := []byte{0, max, current}
	payload := append(append([]byte{0xff, byte(commandType), seq}, multiHeader...), []byte(message)...)
	header := []byte{0x42, 0x45, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(header[2:6], crc32.ChecksumIEEE(payload))
	return append(header, payload...)
}

// createServerMessage creates a server message packet.
func createServerMessage(seq byte) []byte {
	message := fmt.Sprintf("%v %v", testServerMessage, seq)
	payload := append([]byte{0xff, byte(serverMessageType), seq}, []byte(message)...)
	header := []byte{0x42, 0x45, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(header[2:6], crc32.ChecksumIEEE(payload))
	return append(header, payload...)
}
