package web

import (
	"fmt"
	"net/http"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// HTTP-to-HTTPS redirect, hardened against a Host-header open redirect
// (review finding 20b, roadmap chunk 4.2).
//
// The redirect used to build its destination straight from r.Host:
//
//	host := r.Host                                  // attacker-controlled
//	target := fmt.Sprintf("https://%s:%d%s", host, httpsPort, r.RequestURI)
//	http.Redirect(w, r, target, http.StatusMovedPermanently)
//
// The Host header is request input like any other, so anyone could hand out a
// link to the server's own address and have visitors bounced to a host of their
// choosing. That is a phishing primitive wearing the site's domain name, and it
// was served as a 301 -- which browsers and intermediaries cache, so a single
// poisoned response can outlive the request that caused it.
//
// The fix is not to sanitise the header but to stop deriving trust from it. The
// operator has already declared which hostnames this server answers to, for the
// websocket origin check, and that list is exactly the right answer here too:
// same question, same source of truth, no new config.

// redirectTargetHost returns the hostname a plaintext request should be sent
// to, and whether a redirect can be issued at all.
//
// The request's own host is used ONLY when it appears in the operator's
// allow-list, which keeps multi-hostname deployments working. Anything else
// falls back to the configured canonical host, so an unrecognised Host header
// steers nothing.
func redirectTargetHost(requestHost string) (string, bool) {

	allowed := allowedWSOriginHosts()

	if h := hostOnly(requestHost); h != `` {
		for _, a := range allowed {
			if h == a {
				return h, true
			}
		}
	}

	// Not a host we recognise. Fall back to what the operator configured,
	// preferring the web domain, then the MSSP hostname.
	if d := hostOnly(string(configs.GetFilePathsConfig().WebDomain)); d != `` {
		return d, true
	}
	if h := hostOnly(string(configs.GetServerConfig().MSSP.Hostname)); h != `` {
		return h, true
	}

	// Nothing configured and nothing trustworthy in the request. Refusing is
	// the only safe answer: any destination we could invent here would be one
	// the requester chose.
	return ``, false
}

// newHttpsRedirectHandler builds the plaintext-port handler that sends callers
// to the https port.
func newHttpsRedirectHandler(httpsPort int) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		host, ok := redirectTargetHost(r.Host)
		if !ok {
			mudlog.Warn("HTTP", "stage", "https redirect",
				"error", "no trusted host to redirect to",
				"requestHost", r.Host,
				"hint", "set FilePaths.WebDomain to this server's public hostname")
			http.Error(w, "https redirect is not configured for this host", http.StatusBadRequest)
			return
		}

		// r.URL.RequestURI() rather than r.RequestURI. For a proxy-style
		// absolute-form request line the raw field holds a whole URL, which
		// would be pasted after the host and produce a nonsense target; the
		// parsed form is always just path and query.
		target := fmt.Sprintf("https://%s:%d%s", host, httpsPort, r.URL.RequestURI())

		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}
