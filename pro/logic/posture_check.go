package logic

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/biter777/countries"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func AddPostureCheckHook() {
	settings := logic.GetServerSettings()
	interval := time.Hour
	i, err := strconv.Atoi(settings.PostureCheckInterval)
	if err == nil {
		interval = time.Minute * time.Duration(i)
	}
	logic.HookManagerCh <- models.HookDetails{
		Hook:     runPostureChecks,
		Interval: interval,
	}
}
func runPostureChecks() error {
	nets, err := logic.GetNetworks()
	if err != nil {
		return err
	}
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	nodes = logic.AddStaticNodestoList(nodes)
	for _, netI := range nets {
		networkNodes := logic.GetNetworkNodesMemory(nodes, netI.NetID)
		if len(networkNodes) == 0 {
			continue
		}
		pcLi, err := (&schema.PostureCheck{NetworkID: netI.NetID}).ListByNetwork(db.WithContext(context.TODO()))
		if err != nil || len(pcLi) == 0 {
			continue
		}

		for _, nodeI := range networkNodes {
			deviceInfo := models.PostureCheckDeviceInfo{}
			if !nodeI.IsStatic {
				h, err := logic.GetHost(nodeI.HostID.String())
				if err != nil {
					continue
				}
				deviceInfo = models.PostureCheckDeviceInfo{
					ClientLocation: h.CountryCode,
					ClientVersion:  h.Version,
					OS:             h.OS,
					OSVersion:      h.OSVersion,
					OSFamily:       h.OSFamily,
					KernelVersion:  h.KernelVersion,
					AutoUpdate:     h.AutoUpdate,
				}
			} else {
				deviceInfo = models.PostureCheckDeviceInfo{
					ClientLocation: nodeI.StaticNode.Country,
					ClientVersion:  nodeI.StaticNode.ClientVersion,
					OS:             nodeI.StaticNode.OS,
					OSVersion:      nodeI.StaticNode.OSVersion,
					OSFamily:       nodeI.StaticNode.OSFamily,
					KernelVersion:  nodeI.StaticNode.KernelVersion,
				}
			}
			postureChecksViolations, postureCheckVolationSeverityLevel := GetPostureCheckViolations(pcLi, deviceInfo)
			if nodeI.IsStatic {
				extclient, err := logic.GetExtClient(nodeI.StaticNode.ClientID, nodeI.StaticNode.Network)
				if err == nil {
					extclient.PostureChecksViolations = postureChecksViolations
					extclient.PostureCheckVolationSeverityLevel = postureCheckVolationSeverityLevel
					extclient.LastEvaluatedAt = time.Now().UTC()
					logic.SaveExtClient(&extclient)
				}
			} else {
				nodeI.PostureChecksViolations, nodeI.PostureCheckVolationSeverityLevel = postureChecksViolations,
					postureCheckVolationSeverityLevel
				nodeI.LastEvaluatedAt = time.Now().UTC()
				logic.UpsertNode(&nodeI)
			}

		}

	}

	return nil
}

func CheckPostureViolations(d models.PostureCheckDeviceInfo, network models.NetworkID) []models.Violation {
	var violations []models.Violation
	highest := models.SeverityUnknown
	pcLi, err := (&schema.PostureCheck{NetworkID: network.String()}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil || len(pcLi) == 0 {
		return []models.Violation{}
	}
	for _, c := range pcLi {
		// skip disabled checks
		if !c.Status {
			continue
		}

		violated, reason := evaluatePostureCheck(&c, d)
		if !violated {
			continue
		}

		sev := c.Severity
		if sev > highest {
			highest = sev
		}

		v := models.Violation{
			CheckID:   c.ID,
			Name:      c.Name,
			Attribute: string(c.Attribute),
			Message:   reason,
			Severity:  sev,
		}
		violations = append(violations, v)
	}

	return violations
}
func GetPostureCheckViolations(checks []schema.PostureCheck, d models.PostureCheckDeviceInfo) ([]models.Violation, models.Severity) {
	var violations []models.Violation
	highest := models.SeverityUnknown

	for _, c := range checks {
		// skip disabled checks
		if !c.Status {
			continue
		}

		violated, reason := evaluatePostureCheck(&c, d)
		if !violated {
			continue
		}

		sev := c.Severity
		if sev > highest {
			highest = sev
		}

		v := models.Violation{
			CheckID:   c.ID,
			Name:      c.Name,
			Attribute: string(c.Attribute),
			Message:   reason,
			Severity:  sev,
		}
		violations = append(violations, v)
	}

	return violations, highest
}

func evaluatePostureCheck(check *schema.PostureCheck, d models.PostureCheckDeviceInfo) (violated bool, reason string) {
	switch check.Attribute {

	// ------------------------
	// 1. Geographic check
	// ------------------------
	case "client_location":
		if !slices.Contains(check.Values, d.ClientLocation) {
			return true, "client_location not allowed"
		}

	// ------------------------
	// 2. Client version check
	// Supports: exact match OR allowed list OR semver rules
	// ------------------------
	case "client_version":
		for _, rule := range check.Values {
			ok, err := matchVersionRule(d.ClientVersion, rule)
			if err != nil || !ok {
				return true, "client_version violation: " + rule
			}
		}

	// ------------------------
	// 3. OS check
	// ("windows", "mac", "linux", etc.)
	// ------------------------
	case "os":
		if !slices.Contains(check.Values, d.OS) {
			return true, "os not allowed"
		}
	case "os_family":
		if !slices.Contains(check.Values, d.OSFamily) {
			return true, fmt.Sprintf("os_family %q not allowed", d.OSFamily)
		}
	// ------------------------
	// 4. OS version check
	// Supports operators: > >= < <= =
	// ------------------------
	case "os_version":
		for _, rule := range check.Values {
			ok, err := matchVersionRule(d.OSVersion, rule)
			if err != nil || !ok {
				return true, "os_version violation: " + rule
			}
		}
	case "kernel_version":
		for _, rule := range check.Values {
			ok, err := matchVersionRule(d.KernelVersion, rule)
			if err != nil || !ok {
				return true, "kernel_version violation: " + rule
			}
		}
	// ------------------------
	// 5. Auto-update check
	// Values: ["true"] or ["false"]
	// ------------------------
	case "auto_update":
		required := len(check.Values) > 0 && strings.ToLower(check.Values[0]) == "true"
		if required && !d.AutoUpdate {
			return true, "auto_update must be enabled"
		}
		if !required && d.AutoUpdate {
			return true, "auto_update must be disabled"
		}
	}

	return false, ""
}
func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	v = strings.TrimSuffix(v, ",")
	v = strings.TrimSpace(v)
	return v
}

func matchVersionRule(actual, rule string) (bool, error) {
	actual = cleanVersion(actual)
	rule = strings.TrimSpace(rule)

	op := "="

	switch {
	case strings.HasPrefix(rule, ">="):
		op = ">="
		rule = strings.TrimPrefix(rule, ">=")
	case strings.HasPrefix(rule, "<="):
		op = "<="
		rule = strings.TrimPrefix(rule, "<=")
	case strings.HasPrefix(rule, ">"):
		op = ">"
		rule = strings.TrimPrefix(rule, ">")
	case strings.HasPrefix(rule, "<"):
		op = "<"
		rule = strings.TrimPrefix(rule, "<")
	case strings.HasPrefix(rule, "="):
		op = "="
		rule = strings.TrimPrefix(rule, "=")
	}

	rule = cleanVersion(rule)

	cmp := compareVersions(actual, rule)

	switch op {
	case "=":
		return cmp == 0, nil
	case ">":
		return cmp == 1, nil
	case "<":
		return cmp == -1, nil
	case ">=":
		return cmp == 1 || cmp == 0, nil
	case "<=":
		return cmp == -1 || cmp == 0, nil
	}

	return false, fmt.Errorf("invalid rule: %s", rule)
}

func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")

	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}

	for i := 0; i < n; i++ {
		ai, bi := 0, 0

		if i < len(pa) {
			ai, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			bi, _ = strconv.Atoi(pb[i])
		}

		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

func ValidatePostureCheck(pc *schema.PostureCheck) error {
	if pc.Name == "" {
		return errors.New("name cannot be empty")
	}
	_, err := logic.GetNetwork(pc.NetworkID)
	if err != nil {
		return errors.New("invalid network")
	}
	if pc.Attribute != schema.AutoUpdate && pc.Attribute != schema.OS && pc.Attribute != schema.OSVersion &&
		pc.Attribute != schema.ClientLocation &&
		pc.Attribute != schema.ClientVersion {
		return errors.New("unkown attribute")
	}
	if len(pc.Values) == 0 {
		return errors.New("attribute value cannot be empty")
	}
	if len(pc.Tags) > 0 {
		for tagID := range pc.Tags {
			if tagID == "*" {
				continue
			}
			_, err := GetTag(models.TagID(tagID))
			if err != nil {
				return errors.New("unknown tag")
			}
		}
	} else {
		pc.Tags = make(datatypes.JSONMap)
		pc.Tags["*"] = struct{}{}
	}
	if pc.Attribute == schema.ClientLocation {
		for _, loc := range pc.Values {
			if countries.ByName(loc) == countries.Unknown {
				return errors.New("invalid country code")
			}
		}
	}
	return nil
}
