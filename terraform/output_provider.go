package terraform

import "strings"

type Outputs struct {
	ExternalIP             string   `json:"-"`
	NetworkName            string   `json:"-"`
	SubnetworkName         string   `json:"-"`
	BOSHTag                string   `json:"-"`
	InternalTag            string   `json:"-"`
	DirectorAddress        string   `json:"-"`
	RouterBackendService   string   `json:"-"`
	SSHProxyTargetPool     string   `json:"-"`
	TCPRouterTargetPool    string   `json:"-"`
	WSTargetPool           string   `json:"-"`
	ConcourseTargetPool    string   `json:"-"`
	RouterLBIP             string   `json:"cf_router_lb,omitempty"`
	SSHProxyLBIP           string   `json:"cf_ssh_proxy_lb,omitempty"`
	TCPRouterLBIP          string   `json:"cf_tcp_router_lb,omitempty"`
	WebSocketLBIP          string   `json:"cf_websocket_lb,omitempty"`
	ConcourseLBIP          string   `json:"-"`
	SystemDomainDNSServers []string `json:"cf_system_domain_dns_servers,omitempty"`
}

type OutputProvider struct {
	executor executor
}

func NewOutputProvider(executor executor) OutputProvider {
	return OutputProvider{
		executor: executor,
	}
}

func (o OutputProvider) Get(tfState, lbType string, domainExists bool) (Outputs, error) {
	if tfState == "" {
		return Outputs{}, nil
	}

	externalIP, err := o.executor.Output(tfState, "external_ip")
	if err != nil {
		return Outputs{}, err
	}

	networkName, err := o.executor.Output(tfState, "network_name")
	if err != nil {
		return Outputs{}, err
	}

	subnetworkName, err := o.executor.Output(tfState, "subnetwork_name")
	if err != nil {
		return Outputs{}, err
	}

	boshTag, err := o.executor.Output(tfState, "bosh_open_tag_name")
	if err != nil {
		return Outputs{}, err
	}

	internalTag, err := o.executor.Output(tfState, "internal_tag_name")
	if err != nil {
		return Outputs{}, err
	}

	directorAddress, err := o.executor.Output(tfState, "director_address")
	if err != nil {
		return Outputs{}, err
	}

	var (
		routerBackendService      string
		sshProxyTargetPool        string
		tcpRouterTargetPool       string
		wsTargetPool              string
		routerLBIP                string
		sshProxyLBIP              string
		tcpRouterLBIP             string
		webSocketLBIP             string
		systemDomainDNSServersRaw string
		systemDomainDNSServers    []string
	)

	if lbType == "cf" {
		routerBackendService, err = o.executor.Output(tfState, "router_backend_service")
		if err != nil {
			return Outputs{}, err
		}

		sshProxyTargetPool, err = o.executor.Output(tfState, "ssh_proxy_target_pool")
		if err != nil {
			return Outputs{}, err
		}

		tcpRouterTargetPool, err = o.executor.Output(tfState, "tcp_router_target_pool")
		if err != nil {
			return Outputs{}, err
		}

		wsTargetPool, err = o.executor.Output(tfState, "ws_target_pool")
		if err != nil {
			return Outputs{}, err
		}

		routerLBIP, err = o.executor.Output(tfState, "router_lb_ip")
		if err != nil {
			return Outputs{}, err
		}

		sshProxyLBIP, err = o.executor.Output(tfState, "ssh_proxy_lb_ip")
		if err != nil {
			return Outputs{}, err
		}

		tcpRouterLBIP, err = o.executor.Output(tfState, "tcp_router_lb_ip")
		if err != nil {
			return Outputs{}, err
		}

		webSocketLBIP, err = o.executor.Output(tfState, "ws_lb_ip")
		if err != nil {
			return Outputs{}, err
		}

		if domainExists {
			systemDomainDNSServersRaw, err = o.executor.Output(tfState, "system_domain_dns_servers")
			if err != nil {
				return Outputs{}, err
			}

			systemDomainDNSServers = strings.Split(systemDomainDNSServersRaw, ",\n")
		}
	}

	var concourseTargetPool, concourseLBIP string
	if lbType == "concourse" {
		concourseTargetPool, err = o.executor.Output(tfState, "concourse_target_pool")
		if err != nil {
			return Outputs{}, err
		}

		concourseLBIP, err = o.executor.Output(tfState, "concourse_lb_ip")
		if err != nil {
			return Outputs{}, err
		}
	}

	return Outputs{
		ExternalIP:             externalIP,
		NetworkName:            networkName,
		SubnetworkName:         subnetworkName,
		BOSHTag:                boshTag,
		InternalTag:            internalTag,
		DirectorAddress:        directorAddress,
		RouterBackendService:   routerBackendService,
		SSHProxyTargetPool:     sshProxyTargetPool,
		TCPRouterTargetPool:    tcpRouterTargetPool,
		WSTargetPool:           wsTargetPool,
		RouterLBIP:             routerLBIP,
		SSHProxyLBIP:           sshProxyLBIP,
		TCPRouterLBIP:          tcpRouterLBIP,
		WebSocketLBIP:          webSocketLBIP,
		SystemDomainDNSServers: systemDomainDNSServers,
		ConcourseTargetPool:    concourseTargetPool,
		ConcourseLBIP:          concourseLBIP,
	}, nil
}