package plugin

import (
	"github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/world"
)

// connAdapter 将 *net.Conn 适配为 ConnInterface。
type connAdapter struct {
	c *net.Conn
}

func (a *connAdapter) ConnID() int32           { return a.c.ConnID() }
func (a *connAdapter) UUID() string             { return a.c.UUID() }
func (a *connAdapter) USID() string             { return a.c.USID() }
func (a *connAdapter) Name() string             { return a.c.Name() }
func (a *connAdapter) RemoteAddrString() string { return a.c.RemoteAddr().String() }
func (a *connAdapter) IsConnected() bool        { return a.c.IsConnected() }
func (a *connAdapter) TeamID() world.TeamID     { return world.TeamID(a.c.TeamID()) }
func (a *connAdapter) IsAdmin() bool            { return a.c.IsAdmin() }

// WrapConn 将 *net.Conn 包装为 ConnInterface。
func WrapConn(c *net.Conn) ConnInterface {
	if c == nil {
		return nil
	}
	return &connAdapter{c: c}
}

// serverAdapter 将 *net.Server 适配为 ServerInterface。
type serverAdapter struct {
	s *net.Server
}

func (a *serverAdapter) BroadcastChat(message string) {
	a.s.BroadcastChat(message)
}

func (a *serverAdapter) SendChatToConn(connID int32, message string) {
	for _, c := range a.s.ListConnectedConns() {
		if c.ConnID() == connID {
			c.SendChat(message)
			return
		}
	}
}

func (a *serverAdapter) ListConnectedConns() []ConnInterface {
	raw := a.s.ListConnectedConns()
	out := make([]ConnInterface, len(raw))
	for i, c := range raw {
		out[i] = WrapConn(c)
	}
	return out
}

func (a *serverAdapter) KickByID(id int32, reason string) bool {
	return a.s.KickByID(id, reason)
}

func (a *serverAdapter) ConnByID(id int32) ConnInterface {
	for _, c := range a.s.ListConnectedConns() {
		if c.ConnID() == id {
			return WrapConn(c)
		}
	}
	return nil
}

func (a *serverAdapter) ServerName() string {
	name, _, _ := a.s.ServerMeta()
	return name
}

func (a *serverAdapter) PlayerDisplayName(c ConnInterface) string {
	if ca, ok := c.(*connAdapter); ok {
		return a.s.PlayerDisplayName(ca.c)
	}
	return c.Name()
}

func (a *serverAdapter) ReloadWorldLiveForAll() (int, int) {
	return a.s.ReloadWorldLiveForAll()
}

func (a *serverAdapter) OnChat(fn func(connID int32, message string) bool) {
	a.s.AddChatHandler(func(c *net.Conn, msg string) bool {
		return fn(c.ConnID(), msg)
	})
}

func (a *serverAdapter) SendInfoPopup(c ConnInterface, message string, duration float32, align, top, left, bottom, right int32) {
	if nc := unwrapConn(c); nc != nil {
		a.s.SendInfoPopup(nc, message, duration, align, top, left, bottom, right)
	}
}

func (a *serverAdapter) MapName() string {
	if a.s.MapNameFn != nil {
		return a.s.MapNameFn()
	}
	return ""
}

func (a *serverAdapter) GameTimeSeconds() float64 {
	if a.s.StateSnapshotFn != nil {
		if snap := a.s.StateSnapshotFn(); snap != nil {
			return float64(snap.TimeData)
		}
	}
	return 0
}

func (a *serverAdapter) SessionCount() int {
	count := 0
	for _, s := range a.s.ListSessions() {
		if s.Connected {
			count++
		}
	}
	return count
}

func (a *serverAdapter) SendMenu(c ConnInterface, menuID int32, title, message string, options [][]string) {
	if nc := unwrapConn(c); nc != nil {
		a.s.SendMenu(nc, menuID, title, message, options)
	}
}

func (a *serverAdapter) SendInfoMessage(c ConnInterface, message string) {
	if nc := unwrapConn(c); nc != nil {
		a.s.SendInfoMessage(nc, message)
	}
}

func (a *serverAdapter) SendOpenURI(c ConnInterface, uri string) {
	if nc := unwrapConn(c); nc != nil {
		a.s.SendOpenURI(nc, uri)
	}
}

func (a *serverAdapter) SendChat(c ConnInterface, message string) {
	if nc := unwrapConn(c); nc != nil {
		a.s.SendChat(nc, message)
	}
}

func (a *serverAdapter) BroadcastSetHudTextReliable(message string) {
	a.s.BroadcastSetHudTextReliable(message)
}

func (a *serverAdapter) BroadcastHideHudText() {
	a.s.BroadcastHideHudText()
}

// chatAdapter 用于将 ConnInterface 转回 *net.Conn
func unwrapConn(c ConnInterface) *net.Conn {
	if ca, ok := c.(*connAdapter); ok {
		return ca.c
	}
	return nil
}

// WrapServer 将 *net.Server 包装为 ServerInterface。
func WrapServer(s *net.Server) ServerInterface {
	return &serverAdapter{s: s}
}

// worldAdapter 将 *world.World 适配为 WorldInterface。
type worldAdapter struct {
	w *world.World
}

func (a *worldAdapter) Snapshot() world.Snapshot                { return a.w.Snapshot() }
func (a *worldAdapter) SetPaused(paused bool)                    { a.w.SetPaused(paused) }
func (a *worldAdapter) IsPaused() bool                           { return a.w.IsPaused() }
func (a *worldAdapter) SetGameOver(v bool)                       { a.w.SetGameOver(v) }
func (a *worldAdapter) IsGameOver() bool                         { return a.w.IsGameOver() }
func (a *worldAdapter) TickNumber() int64                        { return int64(a.w.Snapshot().Tick) }
func (a *worldAdapter) WaveTime() float32                        { return a.w.Snapshot().WaveTime }
func (a *worldAdapter) CurrentWave() int32                       { return a.w.CurrentWave() }
func (a *worldAdapter) TriggerWave()                             { a.w.TriggerWave() }
func (a *worldAdapter) FillTeamCoreItems(team world.TeamID)      { a.w.FillTeamCoreItems(team) }
func (a *worldAdapter) GetWaveManager() *world.WaveManager       { return a.w.GetWaveManager() }

// WrapWorld 将 *world.World 包装为 WorldInterface。
func WrapWorld(w *world.World) WorldInterface {
	return &worldAdapter{w: w}
}
