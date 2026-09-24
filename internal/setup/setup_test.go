package setup

import (
	"strings"
	"testing"
)

func TestWindowsInstallBatConfiguresDoTForProfile(t *testing.T) {
	content, err := WindowsInstallBat(Params{
		Hostname: "p-abc.dns.lab.markun.com.br",
		IP:       "203.0.113.10",
		Port:     "853",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"set \"TEENDNS_HOST=p-abc.dns.lab.markun.com.br\"",
		"set \"TEENDNS_IP=203.0.113.10\"",
		"set \"TEENDNS_PORT=853\"",
		"netsh dns add global dot=yes",
		"dothost=%TEENDNS_HOST%:%TEENDNS_PORT%",
		"autoupgrade=yes",
		"udpfallback=no",
		"netsh dns add encryption server=%TEENDNS_IP%",
		"Start-Process -FilePath '%~f0' -Verb RunAs",
		"lss 26100",
		"ipconfig /flushdns",
		"Resolve-DnsName",
		"example.com",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("batch content missing %q", want)
		}
	}
	if strings.Contains(content, "\n\n") || !strings.HasPrefix(content, "@echo off\r\n") {
		t.Errorf("batch content must use CRLF line endings, got %q", content[:32])
	}
	if strings.Contains(content, "observe") || strings.Contains(content, "email") {
		t.Errorf("batch must not carry policy or personal data")
	}
}

func TestWindowsInstallBatRejectsMissingResolverIP(t *testing.T) {
	_, err := WindowsInstallBat(Params{Hostname: "p-abc.dns.lab.markun.com.br"})
	if err == nil {
		t.Fatal("expected an error without a resolver IP")
	}
}

func TestWindowsRemoveBatRestoresAutomaticDNS(t *testing.T) {
	content, err := WindowsRemoveBat(Params{
		Hostname: "p-abc.dns.lab.markun.com.br",
		IP:       "203.0.113.10",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"netsh interface ipv4 set dns name=\"%%a\" source=dhcp",
		"set \"TEENDNS_IP=203.0.113.10\"",
		"netsh dns delete encryption server=%TEENDNS_IP%",
		"Start-Process -FilePath '%~f0' -Verb RunAs",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("remove batch content missing %q", want)
		}
	}
	if !strings.HasPrefix(content, "@echo off\r\n") {
		t.Errorf("remove batch must use CRLF line endings")
	}
}

func TestAppleMobileConfigUsesTLSWithServerAddresses(t *testing.T) {
	content, err := AppleMobileConfig(Params{
		Hostname: "p-abc.dns.lab.markun.com.br",
		IP:       "203.0.113.10",
		Port:     "853",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	for _, want := range []string{
		"<key>PayloadType</key>\n      <string>com.apple.dnsSettings.managed</string>",
		"<key>DNSProtocol</key>\n        <string>TLS</string>",
		"<string>p-abc.dns.lab.markun.com.br</string>",
		"<string>203.0.113.10</string>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("mobileconfig missing %q", want)
		}
	}
	repeated, err := AppleMobileConfig(Params{Hostname: "p-abc.dns.lab.markun.com.br", IP: "203.0.113.10"})
	if err != nil {
		t.Fatal(err)
	}
	if string(repeated) != body {
		t.Errorf("same endpoint must produce a stable profile")
	}
}

func TestAppleMobileConfigOmitsServerAddressesWithoutIP(t *testing.T) {
	content, err := AppleMobileConfig(Params{Hostname: "p-abc.dns.lab.markun.com.br"})
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	if strings.Contains(body, "ServerAddresses") {
		t.Errorf("profile without resolver IP must not list server addresses")
	}
	if !strings.Contains(body, "<string>p-abc.dns.lab.markun.com.br</string>") {
		t.Errorf("profile without IP must still set the ServerName")
	}
}

func TestParamsDefaults(t *testing.T) {
	params := Params{Hostname: "home.test"}
	normalized, err := params.normalized()
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Port != "853" || normalized.TestDomain != "example.com" {
		t.Errorf("unexpected defaults: %+v", normalized)
	}
}

func TestParamsRejectsInjection(t *testing.T) {
	params := Params{Hostname: ""}
	if _, err := params.normalized(); err == nil {
		t.Fatal("empty hostname must fail")
	}
	params = Params{Hostname: "ok.test", IP: "1.2.3.4\r\nmalicious"}
	if _, err := params.normalized(); err == nil {
		t.Fatal("newlines in values must fail")
	}
}