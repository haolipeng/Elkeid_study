package main

import (
	"regexp"
	"strings"
)

// LoginEvent represents a parsed SSH login event
type LoginEvent struct {
	Status  string // "true" for accepted, "false" for failed
	Types   string // authentication method (publickey, password, etc.)
	Invalid string // "true" if invalid user, "false" otherwise
	User    string
	SIP     string // source IP
	SPort   string // source port
	Extra   string
}

// CertifyEvent represents a parsed SSH certify event
type CertifyEvent struct {
	Authorized string // the user being authorized to
	Principal  string // the krb5 principal
}

// SSHDParser parses sshd log messages
type SSHDParser struct {
	loginRegex   *regexp.Regexp
	certifyRegex *regexp.Regexp
}

// NewSSHDParser creates a new sshd parser
func NewSSHDParser() *SSHDParser {
	// Pattern based on sshd.pest:
	// login = { authenticated ~ ws ~ method ~ ws ~ "for" ~ ws ~ valid ~ user ~ ws ~ "from" ~ ws ~ sip ~ ws ~ "port" ~ ws ~ sport ~ ws ~ "ssh2" ~ (":")? ~ ws? ~ extra }
	// authenticated = { "Accepted" | "Failed" }
	// valid = @{ "invalid user "? }
	// char = { ASCII_ALPHANUMERIC | "-" | "_" | "." | "@" }
	loginPattern := `^(Accepted|Failed)\s+([a-zA-Z0-9\-_.@]+)\s+for\s+(invalid user\s+)?([a-zA-Z0-9\-_.@]*)\s+from\s+([a-zA-Z0-9\-_.@]+)\s+port\s+([a-zA-Z0-9\-_.@]+)\s+ssh2:?\s*(.*)$`

	// certify = { "Authorized to" ~ ws ~ user ~ ", krb5 principal" ~ ws ~ principal ~ ws ~ "(krb5_kuserok)" }
	certifyPattern := `^Authorized to\s+([a-zA-Z0-9\-_.@]*),\s*krb5 principal\s+([a-zA-Z0-9\-_.@]+)\s+\(krb5_kuserok\)$`

	return &SSHDParser{
		loginRegex:   regexp.MustCompile(loginPattern),
		certifyRegex: regexp.MustCompile(certifyPattern),
	}
}

// ParseLogin attempts to parse a login event from the message
func (p *SSHDParser) ParseLogin(message string) (*LoginEvent, bool) {
	matches := p.loginRegex.FindStringSubmatch(message)
	if matches == nil {
		return nil, false
	}

	status := "false"
	if matches[1] == "Accepted" {
		status = "true"
	}

	invalid := "false"
	if strings.TrimSpace(matches[3]) == "invalid user" {
		invalid = "true"
	}

	return &LoginEvent{
		Status:  status,
		Types:   matches[2],
		Invalid: invalid,
		User:    matches[4],
		SIP:     matches[5],
		SPort:   matches[6],
		Extra:   matches[7],
	}, true
}

// ParseCertify attempts to parse a certify event from the message
func (p *SSHDParser) ParseCertify(message string) (*CertifyEvent, bool) {
	matches := p.certifyRegex.FindStringSubmatch(message)
	if matches == nil {
		return nil, false
	}

	return &CertifyEvent{
		Authorized: matches[1],
		Principal:  matches[2],
	}, true
}
