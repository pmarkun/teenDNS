package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Apple mobile configuration profile. One generator serves iOS and macOS so
// the same behaviour is guaranteed on both, keeping the feature small.
const payloadTop = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>PayloadContent</key>
  <array>
    <dict>
      <key>DNSSettings</key>
      <dict>
        <key>DNSProtocol</key>
        <string>TLS</string>
        <key>ServerName</key>
        <string>{{.Hostname}}</string>
{{.ServerAddresses}}      </dict>
      <key>PayloadDescription</key>
      <string>Usa o DNS do perfil teenDNS com criptografia DNS-over-TLS.</string>
      <key>PayloadDisplayName</key>
      <string>teenDNS (DNS)</string>
      <key>PayloadIdentifier</key>
      <string>com.teendns.dns.{{.Identifier}}</string>
      <key>PayloadType</key>
      <string>com.apple.dnsSettings.managed</string>
      <key>PayloadUUID</key>
      <string>{{.DNSUUID}}</string>
      <key>PayloadVersion</key>
      <integer>1</integer>
    </dict>
  </array>
  <key>PayloadDescription</key>
  <string>Configura este aparelho para usar o DNS seguro do perfil teenDNS.</string>
  <key>PayloadDisplayName</key>
  <string>teenDNS</string>
  <key>PayloadIdentifier</key>
  <string>com.teendns.profile.{{.Identifier}}</string>
  <key>PayloadType</key>
  <string>Configuration</string>
  <key>PayloadUUID</key>
  <string>{{.ProfileUUID}}</string>
  <key>PayloadVersion</key>
  <integer>1</integer>
</dict>
</plist>
`

// AppleMobileConfig renders the plist used to install the profile on iOS and
// macOS. When the resolver IP is empty the profile omits ServerAddresses and
// the operating system resolves the ServerName itself.
func AppleMobileConfig(params Params) ([]byte, error) {
	normalized, err := params.normalized()
	if err != nil {
		return nil, err
	}
	if err := validatePort(normalized.Port); err != nil {
		return nil, err
	}
	values := map[string]string{
		"Hostname": normalized.Hostname,
	}
	if normalized.IP != "" {
		values["ServerAddresses"] = "        <key>ServerAddresses</key>\n        <array>\n          <string>" + escapeXML(normalized.IP) + "</string>\n        </array>\n"
	}
	identifier := shortIdentifier(normalized.Hostname)
	values["Identifier"] = identifier
	values["DNSUUID"] = uuidFromSeed(identifier, 1)
	values["ProfileUUID"] = uuidFromSeed(identifier, 2)

	content := payloadTop
	for placeholder, value := range values {
		content = strings.ReplaceAll(content, "{{."+placeholder+"}}", value)
	}
	if normalized.IP == "" {
		content = strings.ReplaceAll(content, "{{.ServerAddresses}}", "")
	} else {
		content = strings.ReplaceAll(content, "{{.ServerAddresses}}", values["ServerAddresses"])
	}
	if strings.Contains(content, "{{.") {
		return nil, fmt.Errorf("template não totalmente renderizado")
	}
	return []byte(content), nil
}

func escapeXML(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}

// shortIdentifier derives a stable, neutral suffix from the hostname, so
// re-downloading a profile for the same endpoint keeps the same identifiers.
func shortIdentifier(hostname string) string {
	sum := sha256.Sum256([]byte(hostname))
	return hex.EncodeToString(sum[:6])
}

func uuidFromSeed(seed string, variant byte) string {
	sum := sha256.Sum256([]byte(seed + string([]byte{variant})))
	digest := hex.EncodeToString(sum[:16])
	return digest[0:8] + "-" + digest[8:12] + "-" + digest[12:16] + "-" + digest[16:20] + "-" + digest[20:32]
}