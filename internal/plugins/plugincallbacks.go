package plugins

import (
	"net"

	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
)

type PluginCallbacks struct {
	userCommands map[string]usercommands.CommandAccess
	mobCommands  map[string]mobcommands.CommandAccess

	iacHandler   func(uint64, []byte) bool
	onLoad       func()
	onSave       func() error
	onNetConnect func(NetConnection)
}

func newPluginCallbacks() PluginCallbacks {
	return PluginCallbacks{
		userCommands: map[string]usercommands.CommandAccess{},
		mobCommands:  map[string]mobcommands.CommandAccess{},
	}
}

type NetConnection interface {
	IsWebSocket() bool
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Close()
	RemoteAddr() net.Addr
	ConnectionId() uint64
	InputDisabled(setTo ...bool) bool
}

func (c *PluginCallbacks) SetIACHandler(f func(uint64, []byte) bool) {
	c.iacHandler = f
}

func (c *PluginCallbacks) SetOnLoad(f func()) {
	c.onLoad = f
}

// SetOnSave registers the plugin's persistence hook. It returns an error so a
// failed save can be reported: previously the signature was func(), so a plugin
// physically could not tell the autosave that its state did not reach disk, and
// the autosave announced success anyway (review finding 35).
func (c *PluginCallbacks) SetOnSave(f func() error) {
	c.onSave = f
}

func (c *PluginCallbacks) SetOnNetConnect(f func(NetConnection)) {
	c.onNetConnect = f
}
