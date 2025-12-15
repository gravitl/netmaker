package logic

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/biter777/countries"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

var postureCheckMutex = &sync.Mutex{}

func AddPostureCheckHook() {
	settings := logic.GetServerSettings()
	interval := time.Hour
	i, err := strconv.Atoi(settings.PostureCheckInterval)
	if err == nil {
		interval = time.Minute * time.Duration(i)
	}
	logic.HookManagerCh <- models.HookDetails{
		Hook:     logic.WrapHook(RunPostureChecks),
		Interval: interval,
	}
}
func RunPostureChecks() error {
	postureCheckMutex.Lock()
	defer postureCheckMutex.Unlock()
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
			if nodeI.IsStatic && !nodeI.IsUserNode {
				continue
			}
			postureChecksViolations, postureCheckVolationSeverityLevel := GetPostureCheckViolations(pcLi, logic.GetPostureCheckDeviceInfoByNode(&nodeI))
			if nodeI.IsUserNode {
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

func CheckPostureViolations(d models.PostureCheckDeviceInfo, network models.NetworkID) ([]models.Violation, models.Severity) {
	pcLi, err := (&schema.PostureCheck{NetworkID: network.String()}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil || len(pcLi) == 0 {
		return []models.Violation{}, models.SeverityUnknown
	}
	violations, level := GetPostureCheckViolations(pcLi, d)
	return violations, level
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
		if d.IsUser && c.Attribute == schema.AutoUpdate {
			continue
		}
		// Check if tags match
		if !d.IsUser {
			// Check if posture check has wildcard tag - applies to all devices
			if _, hasWildcard := c.Tags["*"]; hasWildcard {
				// Wildcard tag matches all devices, continue to evaluate the check
			} else if len(c.Tags) > 0 {
				// Check has specific tags - device must have at least one matching tag
				if len(d.Tags) == 0 {
					// Device has no tags and check doesn't have wildcard, skip
					continue
				}
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
			} else {
				// Check has no tags configured, skip
				continue
			}
		} else if d.IsUser {
			// Check if posture check has wildcard user group - applies to all users
			if _, hasWildcard := c.UserGroups["*"]; hasWildcard {
				// Wildcard user group matches all users, continue to evaluate the check
			} else if len(c.UserGroups) > 0 {
				// Check has specific user groups - user must have at least one matching group
				if len(d.UserGroups) == 0 {
					// User has no groups and check doesn't have wildcard, skip
					continue
				}
				exists := false
				for userG := range c.UserGroups {
					if _, ok := d.UserGroups[models.UserGroupID(userG)]; ok {
						exists = true
						break
					}
				}
				if !exists {
					continue
				}
			} else {
				// Check has no user groups configured, skip
				continue
			}
		}

		checksByAttribute[c.Attribute] = append(checksByAttribute[c.Attribute], c)
	}

	// Handle OS and OSFamily together with OR logic since they are related
	osChecks := checksByAttribute[schema.OS]
	osFamilyChecks := checksByAttribute[schema.OSFamily]
	if len(osChecks) > 0 || len(osFamilyChecks) > 0 {
		osAllowed := evaluateAttributeChecks(osChecks, d)
		osFamilyAllowed := evaluateAttributeChecks(osFamilyChecks, d)

		// OR condition: if either OS or OSFamily passes, both are considered passed
		if !osAllowed && !osFamilyAllowed {

			// Both failed, add violations for both
			osDenied := getDeniedChecks(osChecks, d)
			osFamilyDenied := getDeniedChecks(osFamilyChecks, d)

			for _, denied := range osDenied {
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
			for _, denied := range osFamilyDenied {
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

	// For all other attributes, check if ANY check allows it
	for attr, attrChecks := range checksByAttribute {
		// Skip OS and OSFamily as they are handled above
		if attr == schema.OS || attr == schema.OSFamily {
			continue
		}

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

// GetPostureCheckDeviceInfoByNode retrieves PostureCheckDeviceInfo for a given node
func GetPostureCheckDeviceInfoByNode(node *models.Node) models.PostureCheckDeviceInfo {
	var deviceInfo models.PostureCheckDeviceInfo

	if !node.IsStatic {
		h, err := logic.GetHost(node.HostID.String())
		if err != nil {
			return deviceInfo
		}
		deviceInfo = models.PostureCheckDeviceInfo{
			ClientLocation: h.CountryCode,
			ClientVersion:  h.Version,
			OS:             h.OS,
			OSVersion:      h.OSVersion,
			OSFamily:       h.OSFamily,
			KernelVersion:  h.KernelVersion,
			AutoUpdate:     h.AutoUpdate,
			Tags:           node.Tags,
		}
	} else if node.IsUserNode {
		deviceInfo = models.PostureCheckDeviceInfo{
			ClientLocation: node.StaticNode.Country,
			ClientVersion:  node.StaticNode.ClientVersion,
			OS:             node.StaticNode.OS,
			OSVersion:      node.StaticNode.OSVersion,
			OSFamily:       node.StaticNode.OSFamily,
			KernelVersion:  node.StaticNode.KernelVersion,
			Tags:           make(map[models.TagID]struct{}),
			IsUser:         true,
			UserGroups:     make(map[models.UserGroupID]struct{}),
		}
		// get user groups
		if node.StaticNode.OwnerID != "" {
			user, err := logic.GetUser(node.StaticNode.OwnerID)
			if err == nil && len(user.UserGroups) > 0 {
				deviceInfo.UserGroups = user.UserGroups
				if user.PlatformRoleID == models.SuperAdminRole || user.PlatformRoleID == models.AdminRole {
					deviceInfo.UserGroups[GetDefaultNetworkAdminGroupID(models.NetworkID(node.Network))] = struct{}{}
					deviceInfo.UserGroups[GetDefaultGlobalAdminGroupID()] = struct{}{}
				}
			}
		}
	}

	return deviceInfo
}

// evaluateAttributeChecks evaluates checks for a specific attribute and returns true if any check allows the device
func evaluateAttributeChecks(attrChecks []schema.PostureCheck, d models.PostureCheckDeviceInfo) bool {
	for _, c := range attrChecks {
		violated, _ := evaluatePostureCheck(&c, d)
		if !violated {
			// At least one check allows it
			return true
		}
	}
	return false
}

// getDeniedChecks returns all checks that denied the device for a specific attribute
func getDeniedChecks(attrChecks []schema.PostureCheck, d models.PostureCheckDeviceInfo) []struct {
	check  schema.PostureCheck
	reason string
} {
	var deniedChecks []struct {
		check  schema.PostureCheck
		reason string
	}

	for _, c := range attrChecks {
		violated, reason := evaluatePostureCheck(&c, d)
		if violated {
			deniedChecks = append(deniedChecks, struct {
				check  schema.PostureCheck
				reason string
			}{check: c, reason: reason})
		}
	}
	return deniedChecks
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
	// Single value representing minimum required version
	// ------------------------
	case schema.ClientVersion:
		if len(check.Values) == 0 {
			return false, ""
		}
		minVersion := check.Values[0]
		cmp := compareVersions(cleanVersion(d.ClientVersion), cleanVersion(minVersion))
		if cmp < 0 {
			return true, fmt.Sprintf("client version '%s' is below minimum required version '%s'", d.ClientVersion, minVersion)
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
	// Single value representing minimum required version
	// ------------------------
	case schema.OSVersion:
		if len(check.Values) == 0 {
			return false, ""
		}
		minVersion := check.Values[0]
		cmp := compareVersions(cleanVersion(d.OSVersion), cleanVersion(minVersion))
		if cmp < 0 {
			return true, fmt.Sprintf("os version '%s' is below minimum required version '%s'", d.OSVersion, minVersion)
		}
	case schema.KernelVersion:
		if len(check.Values) == 0 {
			return false, ""
		}
		minVersion := check.Values[0]
		cmp := compareVersions(cleanVersion(d.KernelVersion), cleanVersion(minVersion))
		if cmp < 0 {
			return true, fmt.Sprintf("kernel version '%s' is below minimum required version '%s'", d.KernelVersion, minVersion)
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
		if len(pc.Values) != 1 {
			return errors.New("version attribute must have exactly one value (minimum version)")
		}
		if !logic.IsValidVersion(pc.Values[0]) {
			return errors.New("invalid attribute version value")
		}
		pc.Values[0] = logic.CleanVersion(pc.Values[0])
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
	}
	if len(pc.UserGroups) > 0 {
		for userGrpID := range pc.UserGroups {
			if userGrpID == "*" {
				continue
			}
			_, err := GetUserGroup(models.UserGroupID(userGrpID))
			if err != nil {
				return errors.New("unknown tag")
			}
		}
	} else {
		pc.UserGroups = make(datatypes.JSONMap)
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
