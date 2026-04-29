package ssrf

import "net"

// IsBlockedIP returns true if the IP is loopback, private, link-local,
// or otherwise not a valid public download target.
func IsBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}
