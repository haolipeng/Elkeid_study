package main

import (
	"testing"
)

func TestParseLogin(t *testing.T) {
	parser := NewSSHDParser()

	tests := []struct {
		name     string
		input    string
		expected *LoginEvent
	}{
		{
			name:  "Accepted publickey",
			input: "Accepted publickey for zhanglei.sec from 10.87.61.221 port 50998 ssh2: RSA SHA256:l9nMCPKgwkWtfRKH4INyvpU3e+PIXtdKsm3jrvXRuMo",
			expected: &LoginEvent{
				Status:  "true",
				Types:   "publickey",
				Invalid: "false",
				User:    "zhanglei.sec",
				SIP:     "10.87.61.221",
				SPort:   "50998",
				Extra:   "RSA SHA256:l9nMCPKgwkWtfRKH4INyvpU3e+PIXtdKsm3jrvXRuMo",
			},
		},
		{
			name:  "Accepted gssapi-with-mic",
			input: "Accepted gssapi-with-mic for zhanglei.sec from 10.2.222.166 port 57302 ssh2",
			expected: &LoginEvent{
				Status:  "true",
				Types:   "gssapi-with-mic",
				Invalid: "false",
				User:    "zhanglei.sec",
				SIP:     "10.2.222.166",
				SPort:   "57302",
				Extra:   "",
			},
		},
		{
			name:  "Failed password",
			input: "Failed password for zhanglei.sec from 10.2.222.166 port 57294 ssh2",
			expected: &LoginEvent{
				Status:  "false",
				Types:   "password",
				Invalid: "false",
				User:    "zhanglei.sec",
				SIP:     "10.2.222.166",
				SPort:   "57294",
				Extra:   "",
			},
		},
		{
			name:  "Failed none for empty user",
			input: "Failed none for  from 10.2.222.166 port 57294 ssh2",
			expected: &LoginEvent{
				Status:  "false",
				Types:   "none",
				Invalid: "false",
				User:    "",
				SIP:     "10.2.222.166",
				SPort:   "57294",
				Extra:   "",
			},
		},
		{
			name:  "Failed password for invalid user",
			input: "Failed password for invalid user zhanglei.sec from 10.2.222.166 port 57294 ssh2",
			expected: &LoginEvent{
				Status:  "false",
				Types:   "password",
				Invalid: "true",
				User:    "zhanglei.sec",
				SIP:     "10.2.222.166",
				SPort:   "57294",
				Extra:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parser.ParseLogin(tt.input)
			if !ok {
				t.Fatalf("ParseLogin failed for: %s", tt.input)
			}

			if result.Status != tt.expected.Status {
				t.Errorf("Status: got %s, want %s", result.Status, tt.expected.Status)
			}
			if result.Types != tt.expected.Types {
				t.Errorf("Types: got %s, want %s", result.Types, tt.expected.Types)
			}
			if result.Invalid != tt.expected.Invalid {
				t.Errorf("Invalid: got %s, want %s", result.Invalid, tt.expected.Invalid)
			}
			if result.User != tt.expected.User {
				t.Errorf("User: got %s, want %s", result.User, tt.expected.User)
			}
			if result.SIP != tt.expected.SIP {
				t.Errorf("SIP: got %s, want %s", result.SIP, tt.expected.SIP)
			}
			if result.SPort != tt.expected.SPort {
				t.Errorf("SPort: got %s, want %s", result.SPort, tt.expected.SPort)
			}
			if result.Extra != tt.expected.Extra {
				t.Errorf("Extra: got %s, want %s", result.Extra, tt.expected.Extra)
			}
		})
	}
}

func TestParseCertify(t *testing.T) {
	parser := NewSSHDParser()

	tests := []struct {
		name     string
		input    string
		expected *CertifyEvent
	}{
		{
			name:  "Authorized to same user",
			input: "Authorized to zhanglei.sec, krb5 principal zhanglei.sec@BYTEDANCE.COM (krb5_kuserok)",
			expected: &CertifyEvent{
				Authorized: "zhanglei.sec",
				Principal:  "zhanglei.sec@BYTEDANCE.COM",
			},
		},
		{
			name:  "Authorized to different user",
			input: "Authorized to tiger, krb5 principal zhanglei.sec@BYTEDANCE.COM (krb5_kuserok)",
			expected: &CertifyEvent{
				Authorized: "tiger",
				Principal:  "zhanglei.sec@BYTEDANCE.COM",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parser.ParseCertify(tt.input)
			if !ok {
				t.Fatalf("ParseCertify failed for: %s", tt.input)
			}

			if result.Authorized != tt.expected.Authorized {
				t.Errorf("Authorized: got %s, want %s", result.Authorized, tt.expected.Authorized)
			}
			if result.Principal != tt.expected.Principal {
				t.Errorf("Principal: got %s, want %s", result.Principal, tt.expected.Principal)
			}
		})
	}
}

func TestParseLoginNoMatch(t *testing.T) {
	parser := NewSSHDParser()

	inputs := []string{
		"Some random log message",
		"Connection closed by 10.2.222.166",
		"",
	}

	for _, input := range inputs {
		_, ok := parser.ParseLogin(input)
		if ok {
			t.Errorf("ParseLogin should not match: %s", input)
		}
	}
}
