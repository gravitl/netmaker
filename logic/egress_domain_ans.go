package logic

import (
	"strings"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// DomainAnsMapFromEgress returns domain -> resolved CIDRs from domain_ans_by_domain.
func DomainAnsMapFromEgress(e schema.Egress) map[string][]string {
	return jsonMapToDomainAnsMap(e.DomainAnsByDomain)
}

// DomainAnsForDomain returns resolved CIDRs for one configured domain.
func DomainAnsForDomain(e schema.Egress, domain string) []string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil
	}
	if ans, ok := DomainAnsMapFromEgress(e)[domain]; ok {
		return append([]string(nil), ans...)
	}
	return nil
}

// FlattenDomainAnsMap returns a de-duplicated union of all resolved CIDRs in the map.
func FlattenDomainAnsMap(m map[string][]string) []string {
	if len(m) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, ans := range m {
		for _, cidr := range ans {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if _, ok := seen[cidr]; ok {
				continue
			}
			seen[cidr] = struct{}{}
			out = append(out, cidr)
		}
	}
	return out
}

// AllDomainAnsFromEgress returns the flattened union of per-domain answers (ACL/routing).
func AllDomainAnsFromEgress(e schema.Egress) []string {
	return FlattenDomainAnsMap(DomainAnsMapFromEgress(e))
}

// HasEgressDomainAns is true when at least one resolved CIDR exists for any domain.
func HasEgressDomainAns(e schema.Egress) bool {
	return len(AllDomainAnsFromEgress(e)) > 0
}

// SetEgressDomainAnsForDomain sets resolved CIDRs for a single domain.
func SetEgressDomainAnsForDomain(e *schema.Egress, domain string, ans []string) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	m := DomainAnsMapFromEgress(*e)
	if m == nil {
		m = make(map[string][]string)
	}
	if domain == "" {
		return
	}
	m[domain] = slicesCloneString(ans)
	e.DomainAnsByDomain = domainAnsMapToJSONMap(m)
}

// SetEgressDomainAnsForDomains assigns the same resolved CIDRs to each domain (e.g. static presets).
func SetEgressDomainAnsForDomains(e *schema.Egress, domains, ans []string) {
	m := make(map[string][]string, len(domains))
	clone := slicesCloneString(ans)
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" {
			continue
		}
		m[d] = append([]string(nil), clone...)
	}
	e.DomainAnsByDomain = domainAnsMapToJSONMap(m)
}

// ClearEgressDomainAns clears per-domain answers.
func ClearEgressDomainAns(e *schema.Egress) {
	e.DomainAnsByDomain = nil
}

func domainAnsMapToJSONMap(m map[string][]string) datatypes.JSONMap {
	if len(m) == 0 {
		return nil
	}
	out := make(datatypes.JSONMap, len(m))
	for k, v := range m {
		if len(v) == 0 {
			out[k] = []string{}
			continue
		}
		items := make([]interface{}, len(v))
		for i, s := range v {
			items[i] = s
		}
		out[k] = items
	}
	return out
}

func jsonMapToDomainAnsMap(j datatypes.JSONMap) map[string][]string {
	if len(j) == 0 {
		return nil
	}
	out := make(map[string][]string, len(j))
	for k, v := range j {
		switch vv := v.(type) {
		case []string:
			out[k] = append([]string(nil), vv...)
		case []interface{}:
			for _, item := range vv {
				if s, ok := item.(string); ok && s != "" {
					out[k] = append(out[k], s)
				}
			}
		}
	}
	return out
}

func slicesCloneString(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}
