package configs

type Network struct {
	MaxTelnetConnections ConfigInt         `yaml:"MaxTelnetConnections"` // Maximum number of telnet connections to accept
	TelnetPort           ConfigSliceString `yaml:"TelnetPort"`           // One or more Ports used to accept telnet connections
	AIPort               ConfigInt         `yaml:"AIPort"`               // Dedicated port for AI clients (0 = disabled)
	MaxHumanConnections  ConfigInt         `yaml:"MaxHumanConnections"`  // Pool limit for human connections
	MaxAIConnections     ConfigInt         `yaml:"MaxAIConnections"`     // Pool limit for AI connections
	AICommandsPerRound   ConfigInt         `yaml:"AICommandsPerRound"`   // Max commands per game round for AI connections
	LocalPort            ConfigInt         `yaml:"LocalPort"`            // Port used for admin connections, localhost only
	HttpPort             ConfigInt         `yaml:"HttpPort"`             // Port used for web requests
	HttpsPort            ConfigInt         `yaml:"HttpsPort"`            // Port used for web https requests
	HttpsRedirect        ConfigBool        `yaml:"HttpsRedirect"`        // If true, http traffic will be redirected to https
	TrustedProxies       ConfigSliceString `yaml:"TrustedProxies"`       // IPs/CIDRs whose X-Forwarded-For header may be believed. Loopback only by default.
	// Extra hostnames permitted to open /ws. Same-origin and the hostnames
	// already configured elsewhere (FilePaths.WebDomain, Server.MSSP.Hostname)
	// are always allowed, so this is only needed when the client is served
	// from a different domain than the MUD.
	AllowedWebSocketOrigins ConfigSliceString `yaml:"AllowedWebSocketOrigins"`
	// HTTP(S) server timeouts, in seconds. Without these a slow client can
	// hold a connection open indefinitely at almost no cost to itself
	// (Slowloris). Applied to both the http and https servers.
	//
	// These do NOT affect websocket (/ws) connections: gorilla/websocket
	// clears the HTTP server's deadlines immediately after hijacking the
	// connection (server.go, "Clear deadlines set by HTTP server"). Verify
	// that still holds before upgrading gorilla.
	//
	// Each falls back to its default when absent or below 1, so a timeout
	// cannot be silently switched off; to effectively disable one, set a
	// very large value.
	HttpReadHeaderTimeoutSeconds ConfigInt `yaml:"HttpReadHeaderTimeoutSeconds"` // Time allowed to send request headers
	HttpReadTimeoutSeconds       ConfigInt `yaml:"HttpReadTimeoutSeconds"`       // Time allowed to read the whole request
	HttpWriteTimeoutSeconds      ConfigInt `yaml:"HttpWriteTimeoutSeconds"`      // Time allowed to write the response
	HttpIdleTimeoutSeconds       ConfigInt `yaml:"HttpIdleTimeoutSeconds"`       // Keep-alive idle time between requests

	AfkSeconds     ConfigInt  `yaml:"AfkSeconds"`     // How long until a player is marked as afk?
	MaxIdleSeconds ConfigInt  `yaml:"MaxIdleSeconds"` // How many seconds a player can go without a command in game before being kicked.
	TimeoutMods    ConfigBool `yaml:"TimeoutMods"`    // Whether to kick admin/mods when idle too long.
	ZombieSeconds  ConfigInt  `yaml:"ZombieSeconds"`  // How many seconds a player will be a zombie allowing them to reconnect.
	LogoutRounds   ConfigInt  `yaml:"LogoutRounds"`   // How many rounds of uninterrupted meditation must be completed to log out.
}

func (n *Network) Validate() {

	// Ignore TelnetPort
	// Ignore LocalPort
	// Ignore TimeoutMods

	if n.MaxTelnetConnections < 1 {
		n.MaxTelnetConnections = 50 // default
	}

	if n.AIPort < 0 {
		n.AIPort = 0 // disabled by default
	}

	if n.MaxHumanConnections < 1 {
		n.MaxHumanConnections = 50 // default
	}

	if n.MaxAIConnections < 1 {
		n.MaxAIConnections = 20 // default
	}

	if n.AICommandsPerRound < 1 {
		n.AICommandsPerRound = 3 // default
	}

	if n.HttpPort < 0 {
		n.HttpPort = 0 // default
	}

	if n.HttpsPort < 0 {
		n.HttpsPort = 0 // default
	}

	// Timeout defaults. ReadHeader is the Slowloris brake and is deliberately
	// the tightest; Write is the loosest because it bounds a whole response.
	if n.HttpReadHeaderTimeoutSeconds < 1 {
		n.HttpReadHeaderTimeoutSeconds = 10
	}

	if n.HttpReadTimeoutSeconds < 1 {
		n.HttpReadTimeoutSeconds = 30
	}

	if n.HttpWriteTimeoutSeconds < 1 {
		n.HttpWriteTimeoutSeconds = 60
	}

	if n.HttpIdleTimeoutSeconds < 1 {
		n.HttpIdleTimeoutSeconds = 120
	}

	// TrustedProxies gates whether X-Forwarded-For is believed at all. The
	// header is attacker-controlled unless the *immediate* TCP peer is a proxy
	// we trust, so an over-broad value here turns a logging annoyance into a
	// ban bypass. Loopback is the only safe default: a remote attacker cannot
	// make their socket peer address be 127.0.0.1 (that needs a completed TCP
	// handshake from the loopback interface), and it is what a same-host
	// reverse proxy such as Caddy actually connects from.
	if len(n.TrustedProxies) == 0 {
		n.TrustedProxies = ConfigSliceString{`127.0.0.1/32`, `::1/128`}
	}

	if n.AfkSeconds < 0 {
		n.AfkSeconds = 0
	}

	if n.MaxIdleSeconds < 0 {
		n.MaxIdleSeconds = 0
	}

	if n.ZombieSeconds < 0 {
		n.ZombieSeconds = 0
	}

	if n.LogoutRounds < 0 {
		n.LogoutRounds = 0 // default
	}

}

func GetNetworkConfig() Network {
	ensureConfigValidated()

	configDataLock.RLock()
	defer configDataLock.RUnlock()
	return configData.Network
}
