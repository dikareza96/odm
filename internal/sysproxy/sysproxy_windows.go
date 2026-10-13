package sysproxy

import "golang.org/x/sys/windows/registry"

// readSystem reads the current user's manual proxy from Internet Options
// (HKCU\...\Internet Settings: ProxyEnable, ProxyServer, ProxyOverride).
func readSystem() Config {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return Config{}
	}
	defer k.Close()
	enabled, _, _ := k.GetIntegerValue("ProxyEnable")
	server, _, _ := k.GetStringValue("ProxyServer")
	override, _, _ := k.GetStringValue("ProxyOverride")
	return Config{Enabled: enabled == 1, Server: server, Override: override}
}
