package pod_info

import "net"

type Address struct {
	IP       net.IP `json:"ip"`
	Hostname string `json:"hostname"`
}
