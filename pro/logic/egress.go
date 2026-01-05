package logic

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
)

func ValidateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	_, err := logic.GetNetwork(e.Network)
	if err != nil {
		return errors.New("failed to get network " + err.Error())
	}

	if !servercfg.IsPro && len(e.Nodes) > 1 {
		return errors.New("can only set one routing node on CE")
	}

	if len(e.Nodes) > 0 {
		for k := range e.Nodes {
			_, err := logic.GetNodeByID(k)
			if err != nil {
				return errors.New("invalid routing node " + err.Error())
			}
		}
	}
	if len(e.Tags) > 0 {
		e.Nodes = make(datatypes.JSONMap)
		for tagID := range e.Tags {
			_, err := GetTag(models.TagID(tagID))
			if err != nil {
				return errors.New("invalid tag " + tagID)
			}
		}
	}
	return nil
}

func RemoveTagFromEgress(net models.NetworkID, tagID models.TagID) {
	eli, _ := (&schema.Egress{Network: net.String()}).ListByNetwork(db.WithContext(context.TODO()))
	for _, eI := range eli {
		if _, ok := eI.Tags[tagID.String()]; ok {
			delete(eI.Tags, tagID.String())
			eI.Update(db.WithContext(context.TODO()))
		}
	}
}

func AssignVirtualRangeToEgress(nw *models.Network, eg *schema.Egress) error {
	if nw == nil {
		return fmt.Errorf("network is nil")
	}
	if eg == nil {
		return fmt.Errorf("egress is nil")
	}
	if !eg.Nat {
		return nil
	}

	// v1: only allocate for virtual NAT mode
	if eg.Mode != schema.VirtualNAT {
		return nil
	}

	// already assigned
	if eg.VirtualRange != "" {
		return nil
	}

	if nw.VirtualNATPoolIPv4 == "" || nw.VirtualNATSitePrefixLenIPv4 == 0 {
		return fmt.Errorf("virtual NAT IPv4 pool not configured for network %s", nw.NetID)
	}

	// load already allocated virtual ranges in this network (read-only)
	var allocated []string
	if err := db.FromContext(context.Background()).Model(&schema.Egress{}).
		Where("network = ? AND virtual_range <> ''", eg.Network).
		Pluck("virtual_range", &allocated).Error; err != nil {
		return err
	}

	// allocate a free prefix from the pool and set on model (no DB write here)
	virtualCIDR, err := allocateNextPrefixDeterministic(
		nw.VirtualNATPoolIPv4,
		nw.VirtualNATSitePrefixLenIPv4,
		allocated,
		eg.ID,
	)
	if err != nil {
		return err
	}

	eg.VirtualRange = virtualCIDR
	return nil
}

func allocateNextPrefixDeterministic(poolCIDR string, sitePrefixLen int, allocated []string, seed string) (string, error) {
	_, pool, err := net.ParseCIDR(strings.TrimSpace(poolCIDR))
	if err != nil {
		return "", fmt.Errorf("invalid pool cidr %q: %w", poolCIDR, err)
	}

	poolPrefixLen, bits := pool.Mask.Size()
	if sitePrefixLen < poolPrefixLen || sitePrefixLen > bits {
		return "", fmt.Errorf("sitePrefixLen %d invalid for pool %s", sitePrefixLen, poolCIDR)
	}

	allocSet := map[string]struct{}{}
	for _, a := range allocated {
		_, an, e := net.ParseCIDR(strings.TrimSpace(a))
		if e == nil && an != nil {
			allocSet[an.String()] = struct{}{}
		}
	}

	total := 1 << uint(sitePrefixLen-poolPrefixLen)
	start := hashIndex(seed, total)

	for i := 0; i < total; i++ {
		idx := (start + i) % total
		cand := nthSubnet(pool, sitePrefixLen, idx)
		if cand == nil {
			continue
		}
		cs := cand.String()
		if _, used := allocSet[cs]; !used {
			return cs, nil
		}
	}

	return "", fmt.Errorf("no available prefixes left in pool %s for /%d", poolCIDR, sitePrefixLen)
}

func nthSubnet(pool *net.IPNet, newPrefixLen int, n int) *net.IPNet {
	if pool == nil {
		return nil
	}

	poolPrefixLen, bits := pool.Mask.Size()
	if newPrefixLen < poolPrefixLen || newPrefixLen > bits || n < 0 {
		return nil
	}

	// base IP as big.Int
	base := ipToBigInt(pool.IP)

	// subnet size = 2^(bits - newPrefixLen)
	size := new(big.Int).Lsh(big.NewInt(1), uint(bits-newPrefixLen))

	// offset = n * size
	offset := new(big.Int).Mul(big.NewInt(int64(n)), size)

	ipInt := new(big.Int).Add(base, offset)
	ip := bigIntToIP(ipInt, bits)

	mask := net.CIDRMask(newPrefixLen, bits)
	return &net.IPNet{IP: ip.Mask(mask), Mask: mask}
}

func ipToBigInt(ip net.IP) *big.Int {
	ip = ip.To16()
	if ip == nil {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(ip)
}

func bigIntToIP(i *big.Int, bits int) net.IP {
	b := i.Bytes()
	byteLen := bits / 8
	if len(b) < byteLen {
		pad := make([]byte, byteLen-len(b))
		b = append(pad, b...)
	}
	ip := net.IP(b)
	if bits == 32 {
		return ip.To4()
	}
	return ip
}

func hashIndex(siteID string, mod int) int {
	if mod <= 1 {
		return 0
	}
	sum := sha1.Sum([]byte(siteID))
	// Use first 4 bytes as uint32
	v := binary.BigEndian.Uint32(sum[:4])
	return int(v % uint32(mod))
}
