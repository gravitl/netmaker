package netmaker

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/request"
	"github.com/gravitl/netmaker/models"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type DNSer interface {
	DNS(context.Context) ([]models.Node, error)
}

// Netmaker is a plugin that talks to netmaker API to retrieve hosts
type Netmaker struct {
	Next plugin.Handler
	Fall fall.F
	// Zone is the zone of the base query in the configuration file
	// this is the global domain.
	Zone string
	// Entries is the cache of hosts informations
	Entries *DNSEntries
	// RefreshDuration is the interval at which we will refresh informations from database
	RefreshDuration time.Duration
	// DNSClient is the interface that allows to retrieve hosts from netmaker server
	DNSClient DNSer
}

// hostAddresses associate a network name and an IP address
type hostAddresses map[string]string

// DNSEntries associate hostname to a map of addresses if the host is in multiple networks
// kube-node1:
//   delivery: 100.96.0.1
//   prod:     100.64.0.5
type DNSEntries map[string]hostAddresses

// Name implements the plugin.Handler interface.
func (nm Netmaker) Name() string {
	return netmakerPluginName
}

type responseQuerier func(request.Request, string, *DNSEntries) ([]dns.RR, error)

// ServeDNS implements the plugin.Handler interface.
func (nm Netmaker) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	a := new(dns.Msg)
	a.SetReply(r)
	a.Compress = true
	a.Authoritative = true

	var f responseQuerier

	switch state.QType() {
	case dns.TypeA:
		f = getResponsesAQueries
	default:
		return plugin.NextOrFailure(nm.Name(), nm.Next, ctx, w, r)
	}

	// extract the zone from the query.
	qn := strings.TrimSuffix(state.QName(), "."+nm.Zone)

	// retrieve list of answers to send to client
	answers, err := f(state, qn, nm.Entries)
	if err != nil {
		log.Debugf("Catched an error from the responder: %v", err)
		return plugin.NextOrFailure(nm.Name(), nm.Next, ctx, w, r)
	}

	a.Answer = answers
	state.SizeAndDo(a)
	w.WriteMsg(a)

	return 0, nil
}

// getResponsesAQueries will return response for query of A type.
func getResponsesAQueries(state request.Request, qn string, dataset *DNSEntries) ([]dns.RR, error) {
	entries, err := getMatchingAEntries(qn, dataset)
	if err != nil {
		return nil, err
	}

	answers := make([]dns.RR, 0)
	for _, entry := range entries {
		rr := new(dns.A)
		rr.Hdr = dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: state.QClass()}
		rr.A = net.ParseIP(entry).To4()
		answers = append(answers, []dns.RR{rr}...)
	}

	return answers, nil
}

// getMatchingAEntries retrieves the hosts matching the query. This function is
// here to eases the tests
func getMatchingAEntries(qn string, dataset *DNSEntries) ([]string, error) {
	// for a legit supported node we should have something like
	// qn = kube-node1.network-name
	parts := strings.Split(strings.TrimSuffix(qn, "."), ".")

	// too short or too long are invalid. The domain is splitted before by the
	// configuration zone. This function should only receive the host and
	// network part.
	if len(parts) != 2 {
		return []string{}, fmt.Errorf("invalid host (%v) with %v parts expected 2 parts (separator is '.')", qn, len(parts))
	}

	name := parts[0]
	network := parts[1]

	entries := make([]string, 0)
	if host, ok := (*dataset)[name]; ok {
		if ip, ok2 := host[network]; ok2 {
			entries = append(entries, ip)
		}
	}

	if len(entries) < 1 {
		return []string{}, fmt.Errorf("no valid response")
	}

	return entries, nil
}

// fetchEntries will retrieve the nodes from the Netmaker API.
func (nm Netmaker) fetchEntries() (DNSEntries, error) {
	ctx := context.Background()
	nodes, err := nm.DNSClient.DNS(ctx)
	if err != nil {
		return nil, err
	}

	entries := make(DNSEntries)
	for _, node := range nodes {
		_, ok := entries[node.Name]
		if !ok {
			entries[node.Name] = make(hostAddresses)
		}
		entries[node.Name][node.Network] = node.Address
	}

	return entries, nil
}

// runFetchEntries is a loop that will fetch nodes list from the Netmaker API
// and store them. It's our cached database.
// every RefreshDuration the cache is updated.
func (nm Netmaker) runFetchEntries(globalEntries *DNSEntries) {
	var err error
	var entriesTemp DNSEntries
	for {
		entriesTemp, err = nm.fetchEntries()
		if err == nil {
			log.Debugf("Successfully retrieved DNSEntries from netmaker API server")
			*globalEntries = entriesTemp
		} else {
			log.Errorf("Failed to retrieve DNSEntries: %v", err)
		}
		time.Sleep(nm.RefreshDuration)
	}
}
