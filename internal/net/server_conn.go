package net

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IYanHua/mdt-server/internal/protocol"
)

type Conn struct {
	net.Conn
	serial                *Serializer
	mu                    sync.Mutex
	id                    int32
	playerID              int32
	udpMu                 sync.RWMutex
	udpSendMu             sync.Mutex
	udpAddr               *net.UDPAddr
	hasBegunConnecting    bool
	hasConnected          bool
	hasDisconnected       bool
	connectTime           time.Time
	rawName               string
	name                  string
	uuid                  string
	usid                  string
	locale                string
	mobile                bool
	versionType           string
	color                 int32
	snapX                 float32
	snapY                 float32
	pointerX              float32
	pointerY              float32
	shooting              bool
	boosting              bool
	typing                bool
	miningTilePos         int32
	dead                  bool
	deathTimer            float32
	lastRespawnCheck      time.Time
	lastSpawnAt           time.Time
	teamID                byte
	AdminManager          *AdminManager
	worldReloadUntil      time.Time
	liveWorldStream       bool
	pendingReloadConfirms int
	pendingReloadRespawns int
	respawnChainActive    bool
	respawnChainName      string
	unitID                int32
	controlBuildPos       int32
	controlBuildActive    bool
	lastRespawnReq        time.Time
	lastSpawnRepairAt     time.Time
	lastDeadIgnoreAt      time.Time
	lastDeadIgnoreLogAt   time.Time
	clientDeadIgnores     int
	building              bool
	selectedBlockID       int16
	selectedRotation      int32
	viewX                 float32
	viewY                 float32
	viewWidth             float32
	viewHeight            float32
	lastClientSnapshot    atomic.Int32
	lastClientSnapshotSet atomic.Bool
	lastClientTimeMs      atomic.Int64
	syncTime              atomic.Int64
	snapshotsSent         atomic.Int32
	entityDebugSent       atomic.Int32
	lastRecvPacketID      int
	lastRecvFrameworkID   int
	closed                chan struct{}
	closeOnce             sync.Once
	postOnce              sync.Once
	onSend                func(obj any, packetID int, frameworkID int, size int)
	streamMu              sync.Mutex
	streams               map[int32]*StreamBuilder
	outHigh               chan any
	outNorm               chan any
	outClosed             chan struct{}
	sendCount             atomic.Int64
	sendErrors            atomic.Int64
	sendQueued            atomic.Int64
	sendQueueFull         atomic.Int64
	bytesSent             atomic.Int64
	udpSent               atomic.Int64
	udpErrors             atomic.Int64
	statsMu               sync.Mutex
	byTypeSent            map[string]int64
	byTypeBytes           map[string]int64
}

type UnitInfo struct {
	ID        int32
	X         float32
	Y         float32
	Health    float32
	MaxHealth float32
	TeamID    byte
	TypeID    int16
}

type ControlledBuildInfo struct {
	Pos    int32
	X      float32
	Y      float32
	TeamID byte
}

type UnitRuntimeState struct {
	Shooting       bool
	Boosting       bool
	UpdateBuilding bool
	MineTilePos    int32
	Plans          []*protocol.BuildPlan
}

const sendWriteTimeout = 3 * time.Second

func NewConn(c net.Conn, serial *Serializer) *Conn {
	conn := &Conn{
		Conn:                c,
		serial:              serial,
		connectTime:         time.Now(),
		closed:              make(chan struct{}),
		lastRecvPacketID:    -1,
		lastRecvFrameworkID: -1,
		streams:             make(map[int32]*StreamBuilder),
		outHigh:             make(chan any, 128),
		outNorm:             make(chan any, 256),
		outClosed:           make(chan struct{}),
		byTypeSent:          make(map[string]int64),
		byTypeBytes:         make(map[string]int64),
		miningTilePos:       invalidTilePos,
	}
	conn.lastClientSnapshot.Store(-1)
	go conn.sendLoop()
	return conn
}

func (c *Conn) syncDue(now time.Time, interval time.Duration) bool {
	if c == nil {
		return false
	}
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	last := c.syncTime.Load()
	if last <= 0 {
		return true
	}
	return now.UnixMilli()-last >= interval.Milliseconds()
}

func (c *Conn) markSyncTime(now time.Time) {
	if c == nil {
		return
	}
	c.syncTime.Store(now.UnixMilli())
}

func (c *Conn) ReadObject() (any, error) {
	for {
		lenbuf := make([]byte, 2)
		if _, err := io.ReadFull(c.Conn, lenbuf); err != nil {
			return nil, err
		}
		n := binary.BigEndian.Uint16(lenbuf)
		payload := make([]byte, n)
		if _, err := io.ReadFull(c.Conn, payload); err != nil {
			return nil, err
		}
		c.lastRecvPacketID = -1
		c.lastRecvFrameworkID = -1
		if len(payload) > 0 {
			if payload[0] == 0xFE {
				if len(payload) > 1 {
					c.lastRecvFrameworkID = int(payload[1])
				}
			} else {
				c.lastRecvPacketID = int(payload[0])
			}
		}
		r := bytesReader(payload)
		obj, err := c.serial.ReadObject(r)
		if err != nil {
			return nil, err
		}
		switch v := obj.(type) {
		case *protocol.StreamBegin:
			c.streamMu.Lock()
			c.streams[v.ID] = NewStreamBuilder(v, c.serial.Registry)
			c.streamMu.Unlock()
			continue
		case *protocol.StreamChunk:
			c.streamMu.Lock()
			sb := c.streams[v.ID]
			if sb != nil {
				sb.Add(v)
				if sb.Done() {
					delete(c.streams, v.ID)
					c.streamMu.Unlock()
					packet, err := sb.Build()
					if err != nil {
						return nil, err
					}
					return packet, nil
				}
			}
			c.streamMu.Unlock()
			continue
		default:
			return obj, nil
		}
	}
}

func (c *Conn) Send(obj any) error {
	return c.sendNow(obj)
}

func (c *Conn) sendNow(obj any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.serial == nil {
		return errors.New("serializer unavailable")
	}

	buf := newBuffer()
	if err := c.serial.WriteObject(buf, obj); err != nil {
		fmt.Printf("[net] sendNow encode failed id=%d err=%v obj=%T\n", c.id, err, obj)
		return err
	}
	payload := buf.Bytes()
	packetID := -1
	frameworkID := -1
	if len(payload) >= 1 {
		if payload[0] == 0xFE {
			if len(payload) >= 2 {
				frameworkID = int(payload[1])
			}
		} else {
			packetID = int(payload[0])
		}
	}
	// Keep per-packet logs off by default to avoid log flood.
	if packetID >= 0 && c.sendCount.Load() < 40 && globalVerboseNetLog.Load() {
		fmt.Printf("[net] tx id=%d packet_id=%d type=%T len=%d\n", c.id, packetID, obj, len(payload))
	}
	if len(payload) > 0xFFFF {
		return fmt.Errorf("payload too large: %d", len(payload))
	}
	lenbuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenbuf, uint16(len(payload)))
	if err := c.Conn.SetWriteDeadline(time.Now().Add(sendWriteTimeout)); err == nil {
		defer func() {
			_ = c.Conn.SetWriteDeadline(time.Time{})
		}()
	}
	if _, err := c.Conn.Write(lenbuf); err != nil {
		c.sendErrors.Add(1)
		return err
	}
	if _, err := c.Conn.Write(payload); err != nil {
		c.sendErrors.Add(1)
		return err
	}
	if c.onSend != nil {
		c.onSend(obj, packetID, frameworkID, len(payload))
	}
	c.sendCount.Add(1)
	c.bytesSent.Add(int64(len(payload)))
	c.recordSend(obj, int64(len(payload)))
	return nil
}

func (c *Conn) Encode(obj any) ([]byte, int, int, error) {
	buf := newBuffer()
	if err := c.serial.WriteObject(buf, obj); err != nil {
		return nil, -1, -1, err
	}
	payload := buf.Bytes()
	packetID := -1
	frameworkID := -1
	if len(payload) >= 1 {
		if payload[0] == 0xFE {
			if len(payload) >= 2 {
				frameworkID = int(payload[1])
			}
		} else {
			packetID = int(payload[0])
		}
	}
	return payload, packetID, frameworkID, nil
}

func (c *Conn) setUDPAddr(addr *net.UDPAddr) {
	c.udpMu.Lock()
	c.udpAddr = addr
	c.udpMu.Unlock()
}

func (c *Conn) UDPAddr() *net.UDPAddr {
	c.udpMu.RLock()
	defer c.udpMu.RUnlock()
	return c.udpAddr
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

func newBuffer() *bytes.Buffer {
	return &bytes.Buffer{}
}

func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.hasDisconnected = true
		c.mu.Unlock()
		close(c.closed)
		close(c.outClosed)
		_ = c.Conn.SetDeadline(time.Now())
		err = c.Conn.Close()
	})
	return err
}

func (c *Conn) PlayerID() int32 {
	return c.playerID
}

func (c *Conn) ConnID() int32 {
	return c.id
}

func (c *Conn) UnitID() int32 {
	return c.unitID
}

func (c *Conn) ControlledBuild() (int32, bool) {
	if c == nil || !c.controlBuildActive {
		return 0, false
	}
	return c.controlBuildPos, true
}

func (c *Conn) TeamID() byte {
	if c == nil {
		return 0
	}
	return c.teamID
}

func (c *Conn) SetTeamID(teamID byte) {
	if c == nil {
		return
	}
	c.teamID = teamID
}

func (c *Conn) UUID() string {
	return c.uuid
}

func (c *Conn) USID() string {
	return c.usid
}

func (c *Conn) Name() string {
	if strings.TrimSpace(c.rawName) != "" {
		return c.rawName
	}
	return c.name
}

func (c *Conn) BaseName() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.rawName)
}

func (c *Conn) VersionType() string {
	return c.versionType
}

func (c *Conn) SnapshotPos() (float32, float32) {
	return c.snapX, c.snapY
}

func (c *Conn) IsConnected() bool {
	if c == nil {
		return false
	}
	return !c.hasDisconnected
}

func (c *Conn) IsBuilding() bool {
	if c == nil {
		return false
	}
	return c.building
}

func (c *Conn) IsDead() bool {
	if c == nil {
		return true
	}
	return c.dead
}

func (c *Conn) MiningTilePos() (int32, bool) {
	if c == nil || c.miningTilePos == invalidTilePos {
		return 0, false
	}
	return c.miningTilePos, true
}

func (c *Conn) SendStream(typeID byte, payload []byte) error {
	begin := &protocol.StreamBegin{
		ID:    rand.Int31(),
		Total: int32(len(payload)),
		Type:  typeID,
	}
	if begin.ID == 0 {
		begin.ID = 1
	}
	if err := c.Send(begin); err != nil {
		return err
	}
	for len(payload) > 0 {
		// Match Mindustry 157 ArcNetProvider InputStreamSender(stream, 1024).
		chunkLen := 1024
		if len(payload) < chunkLen {
			chunkLen = len(payload)
		}
		chunk := &protocol.StreamChunk{
			ID:   begin.ID,
			Data: append([]byte(nil), payload[:chunkLen]...),
		}
		if err := c.Send(chunk); err != nil {
			return err
		}
		payload = payload[chunkLen:]
	}
	return nil
}

func (c *Conn) SendAsync(obj any) error {
	return c.SendAsyncPriority(obj, priorityOf(obj))
}

func (c *Conn) queueReloadConfirm(respawn bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pendingReloadConfirms++
	if respawn {
		c.pendingReloadRespawns++
	}
	c.mu.Unlock()
}

func (c *Conn) SetLiveWorldStream(enabled bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.liveWorldStream = enabled
	c.mu.Unlock()
}

func (c *Conn) UsesLiveWorldStream() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.liveWorldStream
}

func (c *Conn) takeQueuedReloadConfirm() (pending bool, respawn bool) {
	if c == nil {
		return false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pendingReloadConfirms <= 0 {
		return false, false
	}
	c.pendingReloadConfirms--
	if c.pendingReloadRespawns > 0 {
		c.pendingReloadRespawns--
		respawn = true
	}
	return true, respawn
}

func (c *Conn) beginRespawnChain(name string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.respawnChainActive {
		return false
	}
	c.respawnChainActive = true
	c.respawnChainName = name
	return true
}

func (c *Conn) endRespawnChain(name string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.respawnChainActive && (name == "" || c.respawnChainName == "" || c.respawnChainName == name) {
		c.respawnChainActive = false
		c.respawnChainName = ""
	}
	c.mu.Unlock()
}

func (c *Conn) respawnChainInProgress() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.respawnChainActive
}

func (c *Conn) SendAsyncPriority(obj any, prio int) error {
	select {
	case <-c.outClosed:
		return errors.New("connection closed")
	default:
	}
	switch prio {
	case protocol.PriorityHigh:
		select {
		case c.outHigh <- obj:
			c.sendQueued.Add(1)
			return nil
		default:
			c.sendQueueFull.Add(1)
			return c.sendNow(obj)
		}
	default:
		select {
		case c.outNorm <- obj:
			c.sendQueued.Add(1)
			return nil
		default:
			c.sendQueueFull.Add(1)
			return c.sendNow(obj)
		}
	}
}

func (c *Conn) sendLoop() {
	for {
		select {
		case obj := <-c.outHigh:
			if err := c.sendNow(obj); err != nil {
				_ = c.Close()
				return
			}
			continue
		default:
		}

		select {
		case obj := <-c.outHigh:
			if err := c.sendNow(obj); err != nil {
				_ = c.Close()
				return
			}
		case obj := <-c.outNorm:
			if err := c.sendNow(obj); err != nil {
				_ = c.Close()
				return
			}
		case <-c.outClosed:
			return
		}
	}
}

func priorityOf(obj any) int {
	if p, ok := obj.(protocol.Packet); ok {
		return p.Priority()
	}
	return protocol.PriorityNormal
}

func (c *Conn) remoteIP() string {
	if c.Conn == nil || c.Conn.RemoteAddr() == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(c.Conn.RemoteAddr().String())
	if err != nil {
		return c.Conn.RemoteAddr().String()
	}
	return host
}

func (c *Conn) remoteEndpoint() (string, string) {
	if c == nil || c.Conn == nil || c.Conn.RemoteAddr() == nil {
		return "", ""
	}
	host, port, err := net.SplitHostPort(c.Conn.RemoteAddr().String())
	if err != nil {
		return c.Conn.RemoteAddr().String(), ""
	}
	return host, port
}


