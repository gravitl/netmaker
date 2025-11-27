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
		Hook:     RunPostureChecks,
		Interval: interval,
	}
}
func RunPostureChecks() error {
	nets, err := logic.GetNetworks()
	if err != nil {
		return err
	}
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	for _, netI := range nets {
		networkNodes := logic.GetNetworkNodesMemory(nodes, netI.NetID)
		if len(networkNodes) == 0 {
			continue
		}
		networkNodes = logic.AddStaticNodestoList(networkNodes)
		pcLi, err := (&schema.PostureCheck{NetworkID: netI.NetID}).ListByNetwork(db.WithContext(context.TODO()))
		if err != nil || len(pcLi) == 0 {
			continue
		}

		for _, nodeI := range networkNodes {
			var deviceInfo models.PostureCheckDeviceInfo
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
					Tags:           nodeI.Tags,
				}
			} else {
				if nodeI.StaticNode.DeviceID == "" {
					continue
				}
				deviceInfo = models.PostureCheckDeviceInfo{
					ClientLocation: nodeI.StaticNode.Country,
					ClientVersion:  nodeI.StaticNode.ClientVersion,
					OS:             nodeI.StaticNode.OS,
					OSVersion:      nodeI.StaticNode.OSVersion,
					OSFamily:       nodeI.StaticNode.OSFamily,
					KernelVersion:  nodeI.StaticNode.KernelVersion,
					Tags:           nodeI.StaticNode.Tags,
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
	pcLi, err := (&schema.PostureCheck{NetworkID: network.String()}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil || len(pcLi) == 0 {
		return []models.Violation{}
	}
	violations, _ := GetPostureCheckViolations(pcLi, d)
	return violations
}
func GetPostureCheckViolations(checks []schema.PostureCheck, d models.PostureCheckDeviceInfo) ([]models.Violation, models.Severity) {
	var violations []models.Violation
	highest := models.SeverityUnknown

	// Group checks by attribute
	checksByAttribute := make(map[schema.Attribute][]schema.PostureCheck)
	for _, c := range checks {
		// skip disabled checks
		if !c.Status {
			continue
		}
		// Check if tags match
		if _, ok := c.Tags["*"]; !ok {
			exists := false
			for tagID := range c.Tags {
				if _, ok := d.Tags[models.TagID(tagID)]; ok {
					exists = true
					break
				}
			}
			if !exists {
				continue
			}
		}
		checksByAttribute[c.Attribute] = append(checksByAttribute[c.Attribute], c)
	}

	// For each attribute, check if ANY check allows it
	for _, attrChecks := range checksByAttribute {
		// Check if any check for this attribute allows the device
		allowed := false
		var deniedChecks []struct {
			check  schema.PostureCheck
			reason string
		}

		for _, c := range attrChecks {
			violated, reason := evaluatePostureCheck(&c, d)
			if !violated {
				// At least one check allows it
				allowed = true
				break
			}
			// Track denied checks with their reasons for violation reporting
			deniedChecks = append(deniedChecks, struct {
				check  schema.PostureCheck
				reason string
			}{check: c, reason: reason})
		}

		// If no check allows it, add violations for all denied checks
		if !allowed {
			for _, denied := range deniedChecks {
				sev := denied.check.Severity
				if sev > highest {
					highest = sev
				}

				v := models.Violation{
					CheckID:   denied.check.ID,
					Name:      denied.check.Name,
					Attribute: string(denied.check.Attribute),
					Message:   denied.reason,
					Severity:  sev,
				}
				violations = append(violations, v)
			}
		}
	}

	return violations, highest
}

func evaluatePostureCheck(check *schema.PostureCheck, d models.PostureCheckDeviceInfo) (violated bool, reason string) {
	switch check.Attribute {

	// ------------------------
	// 1. Geographic check
	// ------------------------
	case schema.ClientLocation:
		if !slices.Contains(check.Values, strings.ToUpper(d.ClientLocation)) {
			return true, fmt.Sprintf("client location '%s' not allowed", CountryNameFromISO(d.ClientLocation))
		}

	// ------------------------
	// 2. Client version check
	// Supports: exact match OR allowed list OR semver rules
	// ------------------------
	case schema.ClientVersion:
		for _, rule := range check.Values {
			ok, err := matchVersionRule(d.ClientVersion, rule)
			if err != nil || !ok {
				return true, fmt.Sprintf("client version '%s' violation", d.ClientVersion)
			}
		}

	// ------------------------
	// 3. OS check
	// ("windows", "mac", "linux", etc.)
	// ------------------------
	case schema.OS:
		if !slices.Contains(check.Values, d.OS) {
			return true, fmt.Sprintf("client os '%s' not allowed", d.OS)
		}
	case schema.OSFamily:
		if !slices.Contains(check.Values, d.OSFamily) {
			return true, fmt.Sprintf("os family '%s' not allowed", d.OSFamily)
		}
	// ------------------------
	// 4. OS version check
	// Supports operators: > >= < <= =
	// ------------------------
	case schema.OSVersion:
		for _, rule := range check.Values {
			ok, err := matchVersionRule(d.OSVersion, rule)
			if err != nil || !ok {
				return true, fmt.Sprintf("os version '%s' violation", d.OSVersion)
			}
		}
	case schema.KernelVersion:
		for _, rule := range check.Values {
			ok, err := matchVersionRule(d.KernelVersion, rule)
			if err != nil || !ok {
				return true, fmt.Sprintf("kernel version '%s' violation", d.KernelVersion)
			}
		}
	// ------------------------
	// 5. Auto-update check
	// Values: ["true"] or ["false"]
	// ------------------------
	case schema.AutoUpdate:
		required := len(check.Values) > 0 && strings.ToLower(check.Values[0]) == "true"
		if required && !d.AutoUpdate {
			return true, "auto update must be enabled"
		}
		if !required && d.AutoUpdate {
			return true, "auto update must be disabled"
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
	allowedAttrvaluesMap, ok := schema.PostureCheckAttrValuesMap[pc.Attribute]
	if !ok {
		return errors.New("unkown attribute")
	}
	if len(pc.Values) == 0 {
		return errors.New("attribute value cannot be empty")
	}
	for i, valueI := range pc.Values {
		pc.Values[i] = strings.ToLower(valueI)
	}
	if pc.Attribute == schema.ClientLocation {
		for i, loc := range pc.Values {
			if countries.ByName(loc) == countries.Unknown {
				return errors.New("invalid country code")
			}
			pc.Values[i] = strings.ToUpper(loc)
		}
	}
	if pc.Attribute == schema.AutoUpdate || pc.Attribute == schema.OS ||
		pc.Attribute == schema.OSFamily {
		for _, valueI := range pc.Values {
			if _, ok := allowedAttrvaluesMap[valueI]; !ok {
				return errors.New("invalid attribute value")
			}
		}
	}
	if pc.Attribute == schema.ClientVersion || pc.Attribute == schema.OSVersion ||
		pc.Attribute == schema.KernelVersion {
		for i, valueI := range pc.Values {
			if !logic.IsValidVersion(valueI) {
				return errors.New("invalid attribute version value")
			}
			pc.Values[i] = logic.CleanVersion(valueI)
		}
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

	return nil
}

func CountryNameFromISO(code string) string {
	c := countries.ByName(code) // works with ISO2, ISO3, full name
	if c == countries.Unknown {
		return ""
	}
	return c.Info().Name
}
