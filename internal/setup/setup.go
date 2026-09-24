// Package setup generates per-profile device configuration files without
// any personal information: only the DoT hostname (the profile's SNI), the
// public resolver address, the DoT port and a test domain. Everything else —
// labels, rules, groups, emails, admin keys — is deliberately excluded.
package setup

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Params describes the neutral, per-profile values injected into templates.
// Hostname is required; the remaining values fall back to sane defaults.
type Params struct {
	Hostname   string
	IP         string
	Port       string
	TestDomain string
}

func (p Params) normalized() (Params, error) {
	hostname := strings.ToLower(strings.TrimSpace(p.Hostname))
	if hostname == "" {
		return Params{}, errors.New("hostname do perfil é obrigatório")
	}
	for _, value := range []string{hostname, p.IP, p.Port, p.TestDomain} {
		if strings.ContainsAny(value, "\r\n") {
			return Params{}, errors.New("hostname/IP/porta inválidos")
		}
	}
	result := Params{
		Hostname:   hostname,
		IP:         strings.TrimSpace(p.IP),
		Port:       strings.TrimSpace(p.Port),
		TestDomain: strings.TrimSpace(p.TestDomain),
	}
	if result.Port == "" {
		result.Port = "853"
	}
	if result.TestDomain == "" {
		result.TestDomain = "example.com"
	}
	if strings.ContainsAny(result.Hostname, " \t") {
		return Params{}, errors.New("hostname inválido")
	}
	return result, nil
}

// ResolverIP resolves the apex of the DoT hostname suffix (for example
// dns.lab.markun.com.br) to the first public IPv4 address of the resolver.
// It returns an empty string when the suffix cannot be resolved, so callers
// can keep generating Apple profiles without an explicit ServerAddresses.
func ResolverIP(suffix string) string {
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(suffix), "."))
	if name == "" {
		return ""
	}
	addresses, err := net.LookupIP(name)
	if err != nil {
		return ""
	}
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return ""
}

// render substitutes {{.Field}} placeholders with the params' values. Values
// are validated by normalized() before reaching here, so the templates never
// see raw operator input.
func render(template string, params Params) string {
	replacer := strings.NewReplacer(
		"{{.Hostname}}", params.Hostname,
		"{{.IP}}", params.IP,
		"{{.Port}}", params.Port,
		"{{.TestDomain}}", params.TestDomain,
	)
	return replacer.Replace(template)
}

// windowsLineEndings returns Windows batch content with CRLF line endings,
// which cmd.exe expects for reliable label handling.
func windowsLineEndings(content string) string {
	return strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\n", "\r\n")
}

func validatePort(port string) error {
	if len(port) == 0 || len(port) > 5 {
		return fmt.Errorf("porta inválida %q", port)
	}
	for _, char := range port {
		if char < '0' || char > '9' {
			return fmt.Errorf("porta inválida %q", port)
		}
	}
	return nil
}