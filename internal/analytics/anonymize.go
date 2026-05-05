package analytics

import "net"

// AnonymizeIP zeroes the host-specific part of an IP address.
// IPv4: last octet set to 0. IPv6: last 8 bytes set to 0 (preserves /64 prefix).
func AnonymizeIP(ip string) string {
	if ip == "" {
		return ""
	}
	p := net.ParseIP(ip)
	if p == nil {
		return ""
	}
	if p4 := p.To4(); p4 != nil {
		p4[3] = 0
		return p4.String()
	}
	p16 := p.To16()
	for i := 8; i < 16; i++ {
		p16[i] = 0
	}
	return p16.String()
}
