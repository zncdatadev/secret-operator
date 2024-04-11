package pod_info

import "net"

type Address struct {
	IP       net.IP
	Hostname string
}
