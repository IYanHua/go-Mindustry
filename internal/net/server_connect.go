package net

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/devlog"
	"github.com/IYanHua/mdt-server/internal/protocol"
)

func (s *Server) startOfficialConnectConfirmPostConnect(c *Conn) {
	if c == nil {
		return
	}
	c.postOnce.Do(func() {
		c.SetWorldReloadGrace(3 * time.Second)
		s.prepareInitialConnectState(c)
		s.sendInitialPlayerSnapshot(c)
		if s.OnPostConnect != nil {
			go s.OnPostConnect(c)
		}
		go s.postConnectLoop(c)
		s.scheduleInitialConnectRespawn(c)
	})
}

func (s *Server) handleConnectPacket(c *Conn, packet *protocol.ConnectPacket) {
	if s == nil || c == nil || packet == nil {
		return
	}
	if s.DevLogger != nil {
		s.DevLogger.LogConnection("connect packet", c.id, c.remoteIP(), packet.Name, packet.UUID,
			devlog.Int32Fld("version", packet.Version),
			devlog.StringFld("version_type", packet.VersionType),
			devlog.IntFld("mods", len(packet.Mods)))
	} else {
		s.verbosef("[net] connect packet id=%d name=%q version=%d type=%q mods=%d\n",
			c.id, packet.Name, packet.Version, packet.VersionType, len(packet.Mods))
	}

	policy := s.admissionPolicy()
	kick := func(reason protocol.KickReason, label string) {
		s.rejectConnect(c, &reason, "")
		if s.DevLogger != nil {
			s.DevLogger.LogConnection("connect rejected", c.id, c.remoteIP(), packet.Name, packet.UUID,
				devlog.StringFld("reason", label))
		} else {
			fmt.Print(formatConnectRejectLog(c.id, label, nil))
		}
	}
	kickText := func(reason string) {
		s.rejectConnect(c, nil, reason)
		if s.DevLogger != nil {
			s.DevLogger.LogConnection("connect rejected", c.id, c.remoteIP(), packet.Name, packet.UUID,
				devlog.StringFld("reason", reason))
		} else {
			fmt.Print(formatConnectRejectLog(c.id, reason, nil))
		}
	}

	if c.hasBegunConnecting {
		kick(protocol.KickReasonIDInUse, "idInUse")
		return
	}

	versionType := strings.TrimSpace(packet.VersionType)
	if versionType == "" {
		kick(protocol.KickReasonTypeMismatch, "typeMismatch")
		return
	}
	if packet.Version == -1 && !policy.AllowCustomClients {
		kick(protocol.KickReasonCustomClient, "customClient")
		return
	}
	if versionType != "official" && !policy.AllowCustomClients {
		kick(protocol.KickReasonTypeMismatch, "typeMismatch")
		return
	}
	if err := ValidateConnect(packet, s.BuildVersion); err != nil {
		switch {
		case errors.Is(err, ErrClientOutdated):
			kick(protocol.KickReasonClientOutdated, "clientOutdated")
		case errors.Is(err, ErrServerOutdated):
			kick(protocol.KickReasonServerOutdated, "serverOutdated")
		default:
			s.rejectConnect(c, nil, "")
			if s.DevLogger != nil {
				s.DevLogger.LogConnection("connect rejected", c.id, c.remoteIP(), packet.Name, packet.UUID,
					devlog.StringFld("reason", "kick"),
					devlog.StringFld("error", err.Error()))
			} else {
				fmt.Print(formatConnectRejectLog(c.id, "kick", err))
			}
		}
		return
	}

	uuid := strings.TrimSpace(packet.UUID)
	usid := strings.TrimSpace(packet.USID)
	if uuid == "" || usid == "" {
		kick(protocol.KickReasonIDInUse, "idInUse")
		return
	}
	name := packet.Name
	fixedName := FixMindustryPlayerName(name)
	if strings.TrimSpace(fixedName) == "" {
		kick(protocol.KickReasonNameEmpty, "nameEmpty")
		return
	}

	ip := c.remoteIP()
	if admissionSubnetBanned(policy, ip) {
		kickText("banned")
		return
	}
	if admissionNameBanned(policy, name) {
		kickText("banned")
		return
	}
	s.mu.Lock()
	ipReason, ipBanned := s.banIP[ip]
	uuidReason, uuidBanned := s.banUUID[uuid]
	s.mu.Unlock()
	if ipBanned || uuidBanned {
		reason := "banned"
		if uuidBanned && strings.TrimSpace(uuidReason) != "" {
			reason = uuidReason
		} else if ipBanned && strings.TrimSpace(ipReason) != "" {
			reason = ipReason
		}
		kickText(reason)
		return
	}
	if s.isRecentlyKicked(uuid, ip) {
		kick(protocol.KickReasonRecentKick, "recentKick")
		return
	}
	if policy.PlayerLimit > 0 && s.activePlayerCount() >= policy.PlayerLimit && !(s.AdminManager != nil && s.AdminManager.IsOp(uuid)) {
		kick(protocol.KickReasonPlayerLimit, "playerLimit")
		return
	}
	if msg, incompatible := incompatibleModsMessage(policy.ExpectedMods, packet.Mods); incompatible {
		kickText(msg)
		return
	}
	if policy.WhitelistEnabled && !whitelistAllows(policy.Whitelist, uuid, usid) {
		kick(protocol.KickReasonWhitelist, "whitelist")
		return
	}
	if policy.StrictIdentity {
		if reason, duplicate := s.hasDuplicateIdentity(c, uuid, usid, name); duplicate {
			label := "idInUse"
			if reason == protocol.KickReasonNameInUse {
				label = "nameInUse"
			}
			kick(reason, label)
			return
		}
	}

	c.connectTime = time.Now()
	c.uuid = uuid
	c.usid = usid
	c.rawName = fixedName
	c.locale = strings.TrimSpace(packet.Locale)
	if c.locale == "" {
		c.locale = "en"
	}
	c.mobile = packet.Mobile
	c.versionType = versionType
	c.color = packet.Color
	if c.playerID == 0 {
		c.playerID = s.nextPlayerID()
	}
	s.assignConnTeam(c, true)
	c.hasBegunConnecting = true
	s.ensurePlayerEntity(c)
	if s.OnConnectAccepted != nil {
		s.OnConnectAccepted(c, packet)
	}
	s.refreshPlayerDisplayName(c)
	s.emitEvent(c, "connect_packet", fmt.Sprintf("%T", packet), fmt.Sprintf("version=%d type=%s mods=%d", packet.Version, packet.VersionType, len(packet.Mods)))
	if err := s.sendWorldHandshake(c, packet); err != nil {
		if isConnWriteClosed(err) {
			fmt.Printf("[net] world handshake aborted id=%d err=%v\n", c.id, err)
			s.emitEvent(c, "world_handshake_aborted", "", err.Error())
			return
		}
		kickText("world data unavailable")
		fmt.Printf("[net] world handshake failed id=%d err=%v\n", c.id, err)
		return
	}
	s.verbosef("[net] world handshake sent id=%d\n", c.id)
	s.emitEvent(c, "world_handshake_sent", "", "")
}

func (s *Server) startClientSnapshotFallbackPostConnect(c *Conn) {
	if c == nil {
		return
	}
	c.postOnce.Do(func() {
		c.SetWorldReloadGrace(3 * time.Second)
		s.prepareInitialConnectState(c)
		s.sendInitialPlayerSnapshot(c)
		if s.OnPostConnect != nil {
			go s.OnPostConnect(c)
		}
		go s.postConnectLoop(c)
		s.scheduleInitialConnectRespawn(c)
	})
}

func (s *Server) handleOfficialConnectConfirm(c *Conn, _ *protocol.Remote_NetServer_connectConfirm_50) {
	if c == nil {
		return
	}
	wasConnected := c.hasConnected
	c.hasConnected = true
	if !wasConnected {
		s.logPlayerJoinCN(c)
		s.broadcastJoinLeaveChat(c, true)
	}
	s.verbosef("[net] connect confirm id=%d\n", c.id)
	s.emitEvent(c, "connect_confirm", "*protocol.Remote_NetServer_connectConfirm_50", "")
	if !wasConnected {
		c.syncTime.Store(0)
		s.startOfficialConnectConfirmPostConnect(c)
		return
	}
	if pendingReload, reloadRespawn := c.takeQueuedReloadConfirm(); pendingReload {
		c.syncTime.Store(0)
		if s.OnHotReloadConnFn != nil {
			go s.OnHotReloadConnFn(c)
		}
		if reloadRespawn {
			go s.handleMapHotReloadRespawn(c)
		}
		c.SetWorldReloadGrace(2 * time.Second)
	}
}

func (s *Server) handleClientSnapshotConnectFallback(c *Conn, _ *protocol.Remote_NetServer_clientSnapshot_48) {
	if c == nil || c.hasConnected || !s.ClientSnapshotConnectFallbackEnabled() {
		return
	}
	c.hasConnected = true
	s.logPlayerJoinCN(c)
	s.broadcastJoinLeaveChat(c, true)
	s.verbosef("[net] connect confirm via clientSnapshot id=%d\n", c.id)
	s.emitEvent(c, "connect_confirm_client_snapshot", "*protocol.Remote_NetServer_clientSnapshot_48", "")
	s.startClientSnapshotFallbackPostConnect(c)
}

func (s *Server) assignConnTeam(c *Conn, force bool) {
	if s == nil || c == nil {
		return
	}
	if !force && c.teamID != 0 {
		return
	}
	teamID := c.TeamID()
	if s.AssignTeamForConnFn != nil {
		if assigned := s.AssignTeamForConnFn(c); assigned != 0 {
			teamID = assigned
		}
	}
	if teamID == 0 {
		teamID = 1
	}
	c.SetTeamID(teamID)
}

func (s *Server) ConnectedTeamCounts() map[byte]int {
	out := make(map[byte]int)
	if s == nil {
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		if c == nil || !c.hasBegunConnecting {
			continue
		}
		if teamID := c.TeamID(); teamID != 0 {
			out[teamID]++
		}
	}
	return out
}

func (c *Conn) InWorldReloadGrace() bool {
	return c != nil && !c.worldReloadUntil.IsZero() && time.Now().Before(c.worldReloadUntil)
}

func (c *Conn) SetWorldReloadGrace(d time.Duration) {
	if c == nil {
		return
	}
	if d <= 0 {
		c.worldReloadUntil = time.Time{}
		return
	}
	c.worldReloadUntil = time.Now().Add(d)
}

func (s *Server) beginWorldHotReload(c *Conn) {
	if c == nil || c.playerID == 0 {
		return
	}
	s.clearConnControlledBuild(c)
	if c.unitID != 0 {
		s.dropPlayerUnitEntity(c, c.unitID)
		c.unitID = 0
	}
	c.dead = true
	c.deathTimer = 0
	c.syncTime.Store(0)
	c.lastRespawnCheck = time.Now()
	c.lastSpawnAt = time.Time{}
	c.SetWorldReloadGrace(3 * time.Second)
	c.queueReloadConfirm(true)
}

func (s *Server) postConnectLoop(c *Conn) {
	syncInterval := s.syncInterval()
	if syncInterval <= 0 {
		syncInterval = 200 * time.Millisecond
	}
	snapshotTicker := time.NewTicker(snapshotPollInterval(syncInterval))
	defer snapshotTicker.Stop()
	keepAliveTicker := time.NewTicker(3 * time.Second)
	defer keepAliveTicker.Stop()
	for {
		select {
		case now := <-snapshotTicker.C:
			if c == nil || !c.hasConnected || c.playerID == 0 || !c.syncDue(now, syncInterval) {
				continue
			}
			c.markSyncTime(now)

			s.maybeRespawn(c)
			state := buildStateSnapshot(s, c)
			if err := s.sendUnreliable(c, state); err != nil {
				fmt.Printf("[net] state snapshot send failed id=%d err=%v\n", c.id, err)
				s.emitEvent(c, "state_snapshot_send_failed", "*protocol.Remote_NetClient_stateSnapshot_35", err.Error())
				return
			}

			if !c.InWorldReloadGrace() {
				packets, hiddenIDs, err := s.buildEntitySnapshotPacketsForConnCached(c)
				if err != nil {
					fmt.Printf("[net] entity snapshot build failed id=%d err=%v\n", c.id, err)
					s.emitEvent(c, "entity_snapshot_build_failed", "*protocol.Remote_NetClient_entitySnapshot_32", err.Error())
					return
				}
				s.logInitialEntitySnapshotDebug(c, packets)
				for _, packet := range packets {
					if packet == nil {
						continue
					}
					if err := s.sendUnreliable(c, packet); err != nil {
						fmt.Printf("[net] entity snapshot send failed id=%d err=%v\n", c.id, err)
						s.emitEvent(c, "entity_snapshot_send_failed", "*protocol.Remote_NetClient_entitySnapshot_32", err.Error())
						return
					}
				}
				if err := s.sendHiddenSnapshotToConn(c, hiddenIDs); err != nil {
					fmt.Printf("[net] hidden snapshot send failed id=%d err=%v\n", c.id, err)
					s.emitEvent(c, "hidden_snapshot_send_failed", "*protocol.Remote_NetClient_hiddenSnapshot_33", err.Error())
					return
				}
			}
			c.snapshotsSent.Add(1)
		case <-keepAliveTicker.C:
			if err := c.SendAsync(&protocol.KeepAlive{}); err != nil {
				fmt.Printf("[net] keepalive send failed id=%d err=%v\n", c.id, err)
				s.emitEvent(c, "keepalive_send_failed", "*protocol.KeepAlive", err.Error())
				return
			}
		case <-c.closed:
			return
		}
	}
}

func (s *Server) sendHiddenSnapshotToConn(c *Conn, hiddenIDs []int32) error {
	if s == nil || c == nil || len(hiddenIDs) == 0 {
		return nil
	}
	return s.sendUnreliable(c, &protocol.Remote_NetClient_hiddenSnapshot_33{
		Ids: protocol.IntSeq{Items: append([]int32(nil), hiddenIDs...)},
	})
}

