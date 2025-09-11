package utils

import (
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
)

// RetryStrategy specifies a strategy to retry an operation after waiting a while,
// with hooks for successful and unsuccessful (>=max) tries.
type RetryStrategy struct {
	Wait             func(time.Duration)
	WaitTime         time.Duration
	WaitTimeIncrease time.Duration
	MaxTries         int
	Try              func() error
	OnMaxTries       func()
	OnSuccess        func()
}

// DoStrategy does the retry strategy specified in the struct, waiting before retrying an operator,
// up to a max number of tries, and if executes a success "finalizer" operation if a retry is successful
func (rs RetryStrategy) DoStrategy() {
	err := rs.Try()
	if err == nil {
		rs.OnSuccess()
		return
	}

	tries := 1
	for {
		if tries >= rs.MaxTries {
			rs.OnMaxTries()
			return
		}
		rs.Wait(rs.WaitTime)
		if err := rs.Try(); err != nil {
			tries++                            // we tried, increase count
			rs.WaitTime += rs.WaitTimeIncrease // for the next time, sleep more
			continue                           // retry
		}
		rs.OnSuccess()
		return
	}
}

func TraceCaller() {
	// Skip 1 frame to get the caller of this function
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		slog.Debug("Unable to get caller information")
		return
	}

	// Get function name from the program counter (pc)
	funcName := runtime.FuncForPC(pc).Name()

	// Print trace details
	slog.Debug("Called from function: %s\n", "func", funcName)
	slog.Debug("File: %s, Line: %d\n", "file", file, "line", line)
}

// NoEmptyStringToCsv takes a bunch of strings, filters out empty ones and returns a csv version of the string
func NoEmptyStringToCsv(strs ...string) string {
	var sb strings.Builder
	for _, str := range strs {
		trimmedStr := strings.TrimSpace(str)
		if trimmedStr != "" && trimmedStr != "<nil>" {
			if sb.Len() > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(str)
		}
	}
	return sb.String()
}

// GetExtClientEndpoint returns the external client endpoint in the format "host:port" or "[host]:port" for IPv6
func GetExtClientEndpoint(hostIpv4Endpoint, hostIpv6Endpoint net.IP, hostListenPort int) string {
	if hostIpv4Endpoint.To4() == nil {
		return fmt.Sprintf("[%s]:%d", hostIpv6Endpoint.String(), hostListenPort)
	} else {
		return fmt.Sprintf("%s:%d", hostIpv4Endpoint.String(), hostListenPort)
	}
}

// SortIfacesByName sorts a slice of Iface by name in ascending order
func SortIfacesByName(ifaces []models.Iface) {
	sort.Slice(ifaces, func(i, j int) bool {
		return ifaces[i].Name < ifaces[j].Name
	})
}

// CompareIfaces compares two slices of Iface and returns true if they are equal
// Two slices are considered equal if they have the same length and all corresponding
// elements have the same Name, AddressString, and IP address
func CompareIfaces(ifaces1, ifaces2 []models.Iface) bool {
	// Check if lengths are different
	if len(ifaces1) != len(ifaces2) {
		return false
	}

	// Compare each element
	for i := range ifaces1 {
		if !CompareIface(ifaces1[i], ifaces2[i]) {
			return false
		}
	}

	return true
}

// CompareIface compares two individual Iface structs and returns true if they are equal
func CompareIface(iface1, iface2 models.Iface) bool {
	// Compare Name
	if iface1.Name != iface2.Name {
		return false
	}

	// Compare AddressString
	if iface1.AddressString != iface2.AddressString {
		return false
	}

	// Compare IP addresses
	if !iface1.Address.IP.Equal(iface2.Address.IP) {
		return false
	}

	// Compare network masks
	if iface1.Address.Mask.String() != iface2.Address.Mask.String() {
		return false
	}

	return true
}
