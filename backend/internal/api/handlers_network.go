package api

import (
	"net"
	"net/http"
	"strings"
)

// handleLocalIP returns the server's local IP address
func (s *Server) handleLocalIP(w http.ResponseWriter, r *http.Request) {
	ip := getLocalIP()
	if ip == "" {
		ip = "127.0.0.1"
	}

	jsonResponse(w, http.StatusOK, map[string]string{"ip": ip})
}

// getLocalIP returns the server's local IP (method for convenience)
func (s *Server) getLocalIP() string {
	ip := getLocalIP()
	if ip == "" {
		return "localhost"
	}
	return ip
}

// getPort returns the server's port
func (s *Server) getPort() string {
	if s.addr == "" {
		return "8081"
	}
	parts := strings.Split(s.addr, ":")
	if len(parts) > 1 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}
	return "8081"
}

// getLocalIP returns the best guess at the local IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	// Prefer private IPs (192.168.x.x, 10.x.x.x, 172.16-31.x.x)
	var candidates []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				// Check for private IP ranges
				if strings.HasPrefix(ip, "192.168.") ||
					strings.HasPrefix(ip, "10.") ||
					strings.HasPrefix(ip, "172.") {
					candidates = append(candidates, ip)
				}
			}
		}
	}

	if len(candidates) > 0 {
		// Prefer 192.168.x.x if available
		for _, ip := range candidates {
			if strings.HasPrefix(ip, "192.168.") {
				return ip
			}
		}
		return candidates[0]
	}

	return ""
}
