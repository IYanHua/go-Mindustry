package net

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

func (s *Server) addPending(c *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[c.id] = c
}

func (s *Server) nextID() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		id := rand.Int31()
		if id == 0 {
			continue
		}
		if _, ok := s.pending[id]; ok {
			continue
		}
		used := false
		for live := range s.conns {
			if live.id == id {
				used = true
				break
			}
		}
		if used {
			continue
		}
		return id
	}
}

func (s *Server) nextPlayerID() int32 {
	for {
		var id int32
		if s.ReserveUnitIDFn != nil {
			id = s.ReserveUnitIDFn()
		}
		if id <= 0 {
			s.mu.Lock()
			s.playerIDNext++
			id = s.playerIDNext
			s.mu.Unlock()
		}
		if id <= 0 || s.entityIDConflicts(id) {
			continue
		}
		return id
	}
}

func (s *Server) serveUDP(conn *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if s != nil && s.shuttingDown.Load() && isListenerClosed(err) {
				return
			}
			if ne, ok := err.(net.Error); ok && (ne.Timeout() || ne.Temporary()) {
				continue
			}
			return
		}
		b := append([]byte(nil), buf[:n]...)
		s.handleUDPDatagram(conn, addr, b)
	}
}

func (s *Server) handleUDPDatagram(conn *net.UDPConn, addr *net.UDPAddr, b []byte) {
	defer func() {
		if rec := recover(); rec != nil {
			remote := "<nil>"
			if addr != nil {
				remote = addr.String()
			}
			fmt.Printf("[net] udp handler panic remote=%s err=%v\n", remote, rec)
		}
	}()

	// Handle raw framework UDP register first, before normal packet decoding.
	if ru, ok := parseRegisterUDPRaw(b); ok {
		s.handleUDPRegister(addr, ru.ConnectionID)
		return
	}

	s.mu.Lock()
	c := s.byUDP[addr.String()]
	if c != nil {
		select {
		case <-c.closed:
			delete(s.byUDP, addr.String())
			c = nil
		default:
		}
	}
	s.mu.Unlock()

	// Unregistered peers should only send framework discovery / UDP register
	// packets. Drop all other datagrams silently to avoid spending CPU and log
	// bandwidth on random Internet scans hitting the UDP port.
	if c == nil && (len(b) == 0 || b[0] != 0xFE) {
		return
	}

	var (
		obj any
		err error
	)
	if c != nil {
		obj, err = c.serial.ReadObject(bytesReader(b))
	} else {
		obj, err = s.Serial.ReadObject(bytesReader(b))
	}
	if err != nil {
		if c != nil && isIgnorableUDPPacketReadError(c, err) {
			s.emitEvent(c, "udp_read_error_ignored", "", fmt.Sprintf("packet_id=%d framework_id=%d err=%v", c.lastRecvPacketID, c.lastRecvFrameworkID, err))
			return
		}
		key := fmt.Sprintf("udp-read-failed:%s:%v", addr.String(), err)
		if s.shouldLogRepeatingNetEvent(key, 2*time.Second) {
			fmt.Printf("[net] udp read failed remote=%s len=%d err=%v\n", addr.String(), len(b), err)
		}
		return
	}
	switch v := obj.(type) {
	case *protocol.RegisterUDP:
		s.handleUDPRegister(addr, v.ConnectionID)
	case *protocol.DiscoverHost:
		payload := s.buildServerData()
		if len(payload) > 0 {
			_, _ = conn.WriteToUDP(payload, addr)
		}
	default:
		if c != nil {
			s.handlePacket(c, v, false)
		}
	}
}

func isIgnorableUDPPacketReadError(c *Conn, err error) bool {
	if c == nil || err == nil {
		return false
	}
	// Public UDP ports attract random Internet traffic. If the peer has only
	// completed UDP registration but has not begun the official connect flow yet,
	// treat unknown packet IDs as ignorable noise instead of noisy errors.
	if c.hasBegunConnecting {
		return false
	}
	if c.lastRecvPacketID == 3 {
		return false
	}
	return strings.Contains(err.Error(), "unknown packet id")
}

func (s *Server) handleUDPRegister(addr *net.UDPAddr, connectionID int32) {
	var tc *Conn
	var rejected *Conn
	rejectReason := ""
	s.mu.Lock()
	c := s.pending[connectionID]
	pending := c != nil
	if c == nil {
		// Client may retry UDP registration if ACK is lost.
		// In that case connection has already moved out of pending.
		for live := range s.conns {
			if live.id == connectionID {
				c = live
				break
			}
		}
	}
	if c != nil {
		if !udpRegisterMatchesTCP(c, addr) {
			rejected = c
			rejectReason = fmt.Sprintf("ip_mismatch udp=%s tcp_ip=%s", safeUDPRegisterAddr(addr), c.remoteIP())
		} else {
			if pending {
				delete(s.pending, c.id)
			}
			if old := c.UDPAddr(); old != nil {
				delete(s.byUDP, old.String())
			}
			c.setUDPAddr(addr)
			s.byUDP[addr.String()] = c
			tc = c
			fmt.Printf("[net] udp registered remote=%s id=%d\n", addr.String(), c.id)
			if err := s.sendUDPRegisterAck(addr, connectionID); err != nil {
				fmt.Printf("[net] udp register ack failed remote=%s id=%d err=%v\n", addr.String(), c.id, err)
			}
			s.verbosef("[net] udp registered remote=%s id=%d\n", addr.String(), c.id)
		}
	}
	s.mu.Unlock()
	if rejected != nil {
		key := fmt.Sprintf("udp-register-reject:%s:%d", safeUDPRegisterAddr(addr), connectionID)
		if s.shouldLogRepeatingNetEvent(key, 2*time.Second) {
			fmt.Printf("[net] udp register rejected remote=%s id=%d reason=%s\n", safeUDPRegisterAddr(addr), connectionID, rejectReason)
		}
		s.emitEvent(rejected, "udp_register_rejected", "", rejectReason)
		return
	}
	// ArcNet clients treat this TCP framework message as UDP registration completion.
	if tc != nil {
		_ = tc.SendAsync(&protocol.RegisterUDP{ConnectionID: connectionID})
	}
}

func safeUDPRegisterAddr(addr *net.UDPAddr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func udpRegisterMatchesTCP(c *Conn, addr *net.UDPAddr) bool {
	if c == nil || addr == nil || addr.IP == nil {
		return false
	}
	tcpIP := net.ParseIP(c.remoteIP())
	return tcpIP != nil && tcpIP.Equal(addr.IP)
}

func parseRegisterUDPRaw(b []byte) (*protocol.RegisterUDP, bool) {
	if len(b) >= 6 && b[0] == 0xFE && b[1] == protocol.FrameworkRegisterUD {
		id := int32(binary.BigEndian.Uint32(b[2:6]))
		return &protocol.RegisterUDP{ConnectionID: id}, true
	}
	if len(b) >= 8 {
		// Some clients may prefix 2-byte length before framework message.
		if b[2] == 0xFE && b[3] == protocol.FrameworkRegisterUD {
			id := int32(binary.BigEndian.Uint32(b[4:8]))
			return &protocol.RegisterUDP{ConnectionID: id}, true
		}
	}
	return nil, false
}

func (s *Server) sendUDP(addr *net.UDPAddr, obj any) error {
	if s.udpConn == nil || addr == nil {
		return errors.New("udp not ready")
	}
	buf := newBuffer()
	if err := s.Serial.WriteObject(buf, obj); err != nil {
		return err
	}
	_, err := s.udpConn.WriteToUDP(buf.Bytes(), addr)
	return err
}

func (s *Server) sendUDPRegisterAck(addr *net.UDPAddr, id int32) error {
	if s.udpConn == nil || addr == nil {
		return errors.New("udp not ready")
	}
	buf := make([]byte, 6)
	buf[0] = 0xFE
	buf[1] = protocol.FrameworkRegisterUD
	binary.BigEndian.PutUint32(buf[2:], uint32(id))
	_, err := s.udpConn.WriteToUDP(buf, addr)
	return err
}

func (s *Server) buildServerData() []byte {
	s.infoMu.RLock()
	name := s.Name
	description := s.Description
	virtualPlayers := s.VirtualPlayers
	s.infoMu.RUnlock()
	if name == "" {
		name = "mdt-server"
	}
	mapName := "unknown"
	if s.MapNameFn != nil {
		if m := s.MapNameFn(); m != "" {
			mapName = m
		}
	}
	players := len(s.ListConnectedConns()) + int(virtualPlayers)
	if players < 0 {
		players = 0
	}
	wave := int32(1)
	version := int32(s.BuildVersion)
	versionType := "official"
	mode := byte(0)
	playerLimit := int32(0)
	modeName := ""
	port := int16(portFromAddr(s.Addr))

	buf := &bytes.Buffer{}
	writeByteString(buf, name, 100)
	writeByteString(buf, mapName, 64)
	_ = binary.Write(buf, binary.BigEndian, int32(players))
	_ = binary.Write(buf, binary.BigEndian, wave)
	_ = binary.Write(buf, binary.BigEndian, version)
	writeByteString(buf, versionType, 32)
	_ = buf.WriteByte(mode)
	_ = binary.Write(buf, binary.BigEndian, playerLimit)
	writeByteString(buf, description, 100)
	writeByteString(buf, modeName, 50)
	_ = binary.Write(buf, binary.BigEndian, port)
	return buf.Bytes()
}

func (s *Server) SetServerName(name string) {
	s.infoMu.Lock()
	s.Name = name
	s.infoMu.Unlock()
}

func (s *Server) SetServerDescription(desc string) {
	s.infoMu.Lock()
	s.Description = desc
	s.infoMu.Unlock()
}

func (s *Server) SetVirtualPlayers(n int32) {
	s.infoMu.Lock()
	s.VirtualPlayers = n
	s.infoMu.Unlock()
}

func (s *Server) ServerMeta() (name string, desc string, virtual int32) {
	s.infoMu.RLock()
	defer s.infoMu.RUnlock()
	return s.Name, s.Description, s.VirtualPlayers
}

func writeByteString(buf *bytes.Buffer, s string, maxLen int) {
	b := []byte(s)
	if len(b) > maxLen {
		b = b[:maxLen]
	}
	_ = buf.WriteByte(byte(len(b)))
	_, _ = buf.Write(b)
}

func portFromAddr(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return v
}
