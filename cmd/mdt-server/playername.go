package main

import (
	"fmt"
	"strings"
	netserver "github.com/IYanHua/mdt-server/internal/net"
	"github.com/IYanHua/mdt-server/internal/persist"
	"github.com/IYanHua/mdt-server/internal/world"
)

func currentPlayerNamePrefix() string {
	if v := runtimePlayerNamePrefix.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func currentPlayerNameSuffix() string {
	if v := runtimePlayerNameSuffix.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func currentPlayerBoundPrefix() string {
	if v := runtimePlayerBoundPrefix.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func currentPlayerUnboundPrefix() string {
	if v := runtimePlayerUnboundPrefix.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func currentPlayerConnIDSuffixFormat() string {
	if v := runtimePlayerConnIDSuffixFormat.Load(); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return " [gray]{id}[]"
}

func applyIDFormat(format, id string) string {
	if strings.TrimSpace(format) == "" || strings.TrimSpace(id) == "" {
		return ""
	}
	return strings.ReplaceAll(format, "{id}", id)
}

func formatDisplayPlayerNameRaw(name string, c *netserver.Conn, publicStore *persist.PublicConnUUIDStore, identityStore *persist.PlayerIdentityStore) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未知玩家"
	}
	if publicStore == nil {
		publicStore = runtimePublicConnUUIDStore
	}
	if identityStore == nil {
		identityStore = runtimePlayerIdentityStore
	}
	resolver := runtimeBindStatusResolver
	var prefix strings.Builder
	var suffix strings.Builder
	prefix.WriteString(currentPlayerNamePrefix())
	if c != nil {
		connUUID := publicConnUUIDValue(publicStore, c.UUID())
		if runtimePlayerBindPrefixEnabled.Load() {
			if resolver != nil && resolver.Bound(connUUID) {
				prefix.WriteString(currentPlayerBoundPrefix())
			} else {
				prefix.WriteString(currentPlayerUnboundPrefix())
			}
		}
		if rec, ok := identityStore.Lookup(connUUID); ok {
			prefix.WriteString(rec.Prefix)
			if runtimePlayerTitleEnabled.Load() && strings.TrimSpace(rec.Title) != "" {
				prefix.WriteString(rec.Title)
			}
			suffix.WriteString(rec.Suffix)
		}
	}
	if c != nil && runtimePlayerConnIDSuffixEnabled.Load() {
		suffix.WriteString(applyIDFormat(currentPlayerConnIDSuffixFormat(), publicConnIDValue(publicStore, c.UUID(), c.ConnID())))
	}
	suffix.WriteString(currentPlayerNameSuffix())
	return prefix.String() + name + suffix.String()
}

func displayPlayerName(c *netserver.Conn) string {
	if c == nil {
		return "未知玩家"
	}
	name := strings.TrimSpace(c.BaseName())
	if name == "" {
		if c.PlayerID() != 0 {
			name = fmt.Sprintf("player-%d", c.PlayerID())
		}
	}
	name = formatDisplayPlayerNameRaw(name, c, nil, nil)
	if runtimePlayerNameColorEnabled.Load() {
		return netserver.RenderMindustryTextForTerminal(name)
	}
	return netserver.StripMindustryColorTags(name)
}

func blockDisplayName(wld *world.World, blockID int16) string {
	if blockID <= 0 || wld == nil {
		return "空"
	}
	name := strings.TrimSpace(wld.BlockNameByID(blockID))
	if name == "" {
		return fmt.Sprintf("block-%d", blockID)
	}
	return translateBlockNameCN(name)
}

func translateBlockNameCN(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	blockNameTranslationMu.RLock()
	cn, ok := blockNameTranslations[n]
	blockNameTranslationMu.RUnlock()
	if ok {
		return cn
	}
	return n
}

