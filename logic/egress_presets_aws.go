package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
)

// awsIPRangesDoc is the public AWS ip-ranges.json document.
type awsIPRangesDoc struct {
	Prefixes     []awsIPPrefix `json:"prefixes"`
	IPv6Prefixes []awsIPPrefix `json:"ipv6_prefixes"`
}

type awsIPPrefix struct {
	IPPrefix            string `json:"ip_prefix"`
	IPv6Prefix          string `json:"ipv6_prefix"`
	Region              string `json:"region"`
	Service             string `json:"service"`
	NetworkBorderGroup  string `json:"network_border_group"`
}

func resolveAWSPresetCIDRs(client *http.Client, p models.EgressPresetApp) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, awsIPRangesURL, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aws ip-ranges: status %d", res.StatusCode)
	}
	var doc awsIPRangesDoc
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, err
	}
	var out []string
	switch {
	case p.ID == "aws-cloudfront-global":
		for _, e := range doc.Prefixes {
			if e.Service == "CLOUDFRONT" && (e.Region == "GLOBAL" || strings.EqualFold(e.Region, "global")) {
				if e.IPPrefix != "" {
					out = append(out, e.IPPrefix)
				}
			}
		}
		for _, e := range doc.IPv6Prefixes {
			if e.Service == "CLOUDFRONT" && (e.Region == "GLOBAL" || strings.EqualFold(e.Region, "global")) {
				if e.IPv6Prefix != "" {
					out = append(out, e.IPv6Prefix)
				}
			}
		}
	case p.ID == "aws-s3-global":
		for _, e := range doc.Prefixes {
			if e.Service == "S3" && e.IPPrefix != "" {
				out = append(out, e.IPPrefix)
			}
		}
		for _, e := range doc.IPv6Prefixes {
			if e.Service == "S3" && e.IPv6Prefix != "" {
				out = append(out, e.IPv6Prefix)
			}
		}
	case strings.HasPrefix(p.ID, "aws-s3-"):
		region := strings.TrimPrefix(p.ID, "aws-s3-")
		for _, e := range doc.Prefixes {
			if e.Service == "S3" && e.Region == region && e.IPPrefix != "" {
				out = append(out, e.IPPrefix)
			}
		}
		for _, e := range doc.IPv6Prefixes {
			if e.Service == "S3" && e.Region == region && e.IPv6Prefix != "" {
				out = append(out, e.IPv6Prefix)
			}
		}
	case strings.HasPrefix(p.ID, "aws-ec2-"):
		region := strings.TrimPrefix(p.ID, "aws-ec2-")
		services := map[string]struct{}{"EC2": {}, "ELB": {}, "ELBV2": {}}
		for _, e := range doc.Prefixes {
			if _, ok := services[e.Service]; !ok {
				continue
			}
			if e.Region != region || e.IPPrefix == "" {
				continue
			}
			out = append(out, e.IPPrefix)
		}
		for _, e := range doc.IPv6Prefixes {
			if _, ok := services[e.Service]; !ok {
				continue
			}
			if e.Region != region || e.IPv6Prefix == "" {
				continue
			}
			out = append(out, e.IPv6Prefix)
		}
	default:
		return nil, fmt.Errorf("unhandled AWS preset %q", p.ID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no CIDRs matched for AWS preset %q", p.ID)
	}
	return UniqueIPNetStrList(out), nil
}
