package tigerbeetle

import (
	"context"
	"fmt"
	"net"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// ResolveAddresses turns "host:port" entries into "ip:port". The TigerBeetle client accepts IP
// addresses only, but in Docker Compose or Kubernetes the replica is usually reachable by name,
// so names are resolved once at startup. A bare port or a bare IP is passed through unchanged.
func ResolveAddresses(ctx context.Context, addresses []string) ([]string, error) {
	resolved := make([]string, 0, len(addresses))
	for _, address := range addresses {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			// Not host:port, for example a bare port such as "3000"; let the client validate it.
			resolved = append(resolved, address)
			continue
		}
		if host == "" || net.ParseIP(host) != nil {
			resolved = append(resolved, address)
			continue
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("tigerbeetle: resolve %q: %w: %w", host, ledger.ErrUnavailable, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("tigerbeetle: resolve %q: %w: no addresses", host, ledger.ErrUnavailable)
		}
		ip := ips[0].IP
		for _, candidate := range ips {
			if v4 := candidate.IP.To4(); v4 != nil {
				ip = v4
				break
			}
		}
		resolved = append(resolved, net.JoinHostPort(ip.String(), port))
	}
	return resolved, nil
}
