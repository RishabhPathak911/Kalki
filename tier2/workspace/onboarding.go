package workspace

import (
	"net"
	"net/url"
	"strings"
)

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func publicCommands(origin, tenant, region, collector, token string) map[string]string {
	u, _ := url.Parse(origin)
	port := u.Port()
	if port == "" {
		port = "443"
	}
	probe := net.JoinHostPort(u.Hostname(), port)
	flags := []string{"--tenant=" + tenant, "--region=" + region, "--collector-id=" + collector, "--tier2-url=" + origin, "--token=" + token, "--probe-target=" + probe, "--window=2s", "--key-file=data/" + tenant + ".key", "--wal-path=data/" + collector + ".wal"}
	for i := range flags {
		flags[i] = quote(flags[i])
	}
	args := strings.Join(flags, " ")
	return map[string]string{"token": token, "collector_id": collector, "collector_command": "./collector " + args, "install_command": "curl --fail --silent --show-error --proto '=https' " + quote(origin+"/install.sh") + " | sh -s -- --base-url " + quote(origin) + " " + args, "windows_install_command": "$s = irm " + psQuote(origin+"/install.ps1") + "; & ([scriptblock]::Create($s)) -BaseUrl " + psQuote(origin) + " -Tenant " + psQuote(tenant) + " -Region " + psQuote(region) + " -CollectorId " + psQuote(collector) + " -Token " + psQuote(token) + " -ProbeTarget " + psQuote(probe) + " -Window '2s'"}
}
