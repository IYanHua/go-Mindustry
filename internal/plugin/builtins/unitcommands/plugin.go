package unitcommands

import (
	"github.com/IYanHua/mdt-server/internal/plugin"
)

// Plugin wraps the AI unit command service as a plugin.
type Plugin struct {
	Svc *Service
}

func (p *Plugin) ID() string { return "builtins/unitcommands" }

func (p *Plugin) Init(ctx *plugin.Context) error {
	if p.Svc == nil {
		p.Svc = NewService()
	}
	return nil
}

func (p *Plugin) Start() error { return nil }
func (p *Plugin) Stop() error  { return nil }
