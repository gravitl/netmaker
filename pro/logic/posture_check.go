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
func RemoveTagFromPostureChecks(tagID models.TagID, netID schema.NetworkID) {
	pcLi, err := (&schema.PostureCheck{NetworkID: netID}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil || len(pcLi) == 0 {
		return
	}
	for _, pcI := range pcLi {
		if _, ok := pcI.Tags[tagID.String()]; ok {
			delete(pcI.Tags, tagID.String())
			pcI.Update(db.WithContext(context.TODO()))
		}
	}
}
func RemoveUserGroupFromPostureChecks(grpID schema.UserGroupID, netID schema.NetworkID) {
	pcLi, err := (&schema.PostureCheck{NetworkID: netID}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil || len(pcLi) == 0 {
		return
	}
	for _, pcI := range pcLi {
		if _, ok := pcI.UserGroups[grpID.String()]; ok {
			delete(pcI.UserGroups, grpID.String())
			pcI.Update(db.WithContext(context.TODO()))
		}
	}
}
func RunPostureChecks() error {
	if !GetFeatureFlags().EnablePostureChecks {
		return nil
	}
	postureCheckMutex.Lock()
	defer postureCheckMutex.Unlock()
	// Refresh MDM device state before evaluating; a no-op when no provider
	// is configured. Errors are already logged inside; we don't want a
	// remote-API hiccup to block the rest of the posture cycle.
	_ = RunMDMSync(context.TODO())
	nets, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return err
	}
	for _, netI := range nets {
		networkNodes := logic.GetNetworkNodesMemory(nodes, netI.Name)
		if len(networkNodes) == 0 {
			continue
		}
		networkNodes = logic.AddStaticNodestoList(networkNodes)
		pcLi, err := (&schema.PostureCheck{NetworkID: schema.NetworkID(netI.Name)}).ListByNetwork(db.WithContext(context.TODO()))
		if err != nil {
			continue
		}
		noChecks := len(pcLi) == 0

		for _, nodeI := range networkNodes {
			if nodeI.IsStatic && !nodeI.IsUserNode {
				continue
			}
			deviceInfo := logic.GetPostureCheckDeviceInfoByNode(&nodeI)
			var postureChecksViolations []models.Violation
			var postureCheckVolationSeverityLevel schema.Severity
			if noChecks {
				postureCheckVolationSeverityLevel = schema.SeverityUnknown
			} else {
				postureChecksViolations, postureCheckVolationSeverityLevel = GetPostureCheckViolations(pcLi, deviceInfo)
			}
			if nodeI.IsUserNode {
				extclient, err := logic.GetExtClient(nodeI.StaticNode.ClientID, nodeI.StaticNode.Network)
				if err == nil {
					if noChecks && len(extclient.PostureChecksViolations) == 0 {
						continue
					}
					emitNewMDMViolationEvents(extclient.PostureChecksViolations, postureChecksViolations, deviceInfo, schema.NetworkID(netI.Name))
					extclient.PostureChecksViolations = postureChecksViolations
					extclient.PostureCheckVolationSeverityLevel = postureCheckVolationSeverityLevel
					extclient.LastEvaluatedAt = time.Now().UTC()
					logic.SaveExtClient(&extclient)
				}
			} else {
				if noChecks && len(nodeI.PostureChecksViolations) == 0 {
					continue
				}
				emitNewMDMViolationEvents(nodeI.PostureChecksViolations, postureChecksViolations, deviceInfo, schema.NetworkID(netI.Name))
				nodeI.PostureChecksViolations, nodeI.PostureCheckVolationSeverityLevel = postureChecksViolations,
					postureCheckVolationSeverityLevel
				nodeI.LastEvaluatedAt = time.Now().UTC()
				logic.UpsertNode(&nodeI)
			}

		}

	}

	return nil
}

func CheckPostureViolations(d models.PostureCheckDeviceInfo, network schema.NetworkID) ([]models.Violation, schema.Severity) {
	if !GetFeatureFlags().EnablePostureChecks {
		return []models.Violation{}, schema.SeverityUnknown
	}
	pcLi, err := (&schema.PostureCheck{NetworkID: network}).ListByNetwork(db.WithContext(context.TODO()))
	if err != nil || len(pcLi) == 0 {
		return []models.Violation{}, schema.SeverityUnknown
	}
	violations, level := GetPostureCheckViolations(pcLi, d)
	return violations, level
}
func GetPostureCheckViolations(checks []schema.PostureCheck, d models.PostureCheckDeviceInfo) ([]models.Violation, schema.Severity) {
	if !GetFeatureFlags().EnablePostureChecks {
		return []models.Violation{}, schema.SeverityUnknown
	}
	var violations []models.Violation
	highest := schema.SeverityUnknown

	// Group checks by attribute
	checksByAttribute := make(map[schema.Attribute][]schema.PostureCheck)
	for _, c := range checks {
		// skip disabled checks
		if !c.Status {
			continue
		}
		if c.Attribute == schema.AutoUpdate && (d.IsUser || d.SkipAutoUpdate) {
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
					if _, ok := d.UserGroups[schema.UserGroupID(userG)]; ok {
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
		h := &schema.Host{
			ID: node.HostID,
		}
		err := h.Get(db.WithContext(context.TODO()))
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
			HostID:         h.ID.String(),
		}
		// Attach the MDM snapshot for the currently configured provider, if any.
		settings := logic.GetServerSettings()
		if settings.MDMProvider != "" {
			state := &schema.DeviceMDMState{HostID: h.ID.String(), Provider: settings.MDMProvider}
			if err := state.Get(db.WithContext(context.TODO())); err == nil {
				deviceInfo.MDMState = state
			}
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
			UserGroups:     make(map[schema.UserGroupID]struct{}),
		}
		// get user groups
		if node.StaticNode.OwnerID != "" {
			user := &schema.User{Username: node.StaticNode.OwnerID}
			err := user.Get(db.WithContext(context.TODO()))
			if err == nil && len(user.UserGroups.Data()) > 0 {
				deviceInfo.UserGroups = user.UserGroups.Data()
				if user.PlatformRoleID == schema.SuperAdminRole || user.PlatformRoleID == schema.AdminRole {
					deviceInfo.UserGroups[GetDefaultNetworkAdminGroupID(schema.NetworkID(node.Network))] = struct{}{}
					deviceInfo.UserGroups[GetDefaultGlobalAdminGroupID()] = struct{}{}
				} else if _, ok := user.UserGroups.Data()[GetDefaultGlobalAdminGroupID()]; ok {

					deviceInfo.UserGroups[GetDefaultNetworkAdminGroupID(schema.NetworkID(node.Network))] = struct{}{}

				} else if _, ok := user.UserGroups.Data()[GetDefaultGlobalUserGroupID()]; ok {

					deviceInfo.UserGroups[GetDefaultNetworkUserGroupID(schema.NetworkID(node.Network))] = struct{}{}
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

	// ------------------------
	// 6. MDM compliance check
	// Config: {require_enrolled, require_compliant, max_state_age_hours}
	// ------------------------
	case schema.MDMCompliance:
		cfg := ParseMDMComplianceConfig(check.Config)
		if d.MDMState == nil {
			return true, "no_mdm_state_for_host"
		}
		if cfg.RequireEnrolled && !d.MDMState.Enrolled {
			return true, "device_not_mdm_enrolled"
		}
		if cfg.RequireCompliant && !d.MDMState.Compliant {
			return true, "device_not_mdm_compliant"
		}
		if cfg.MaxStateAgeHours > 0 &&
			time.Since(d.MDMState.LastSyncedAt) > time.Duration(cfg.MaxStateAgeHours)*time.Hour {
			return true, "mdm_state_stale"
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

// PopulatePostureCheckGroupNames sets group name as the value for each user group key
func PopulatePostureCheckGroupNames(pcs []schema.PostureCheck) {
	for i := range pcs {
		for groupID := range pcs[i].UserGroups {
			if groupID == "*" {
				pcs[i].UserGroups[groupID] = "*"
				continue
			}
			grp, err := logic.GetUserGroup(schema.UserGroupID(groupID))
			if err == nil {
				pcs[i].UserGroups[groupID] = grp.Name
			} else {
				pcs[i].UserGroups[groupID] = groupID
			}
		}
	}
}

func ValidatePostureCheck(pc *schema.PostureCheck) error {
	if pc.Name == "" {
		return errors.New("name cannot be empty")
	}
	err := (&schema.Network{Name: pc.NetworkID.String()}).Get(db.WithContext(context.TODO()))
	if err != nil {
		return errors.New("invalid network")
	}
	_, ok := schema.PostureCheckAttrValuesMap[pc.Attribute]
	if !ok {
		return errors.New("unkown attribute")
	}
	// MDMCompliance uses Config, not Values. Validate the Config payload and
	// short-circuit the Values flow (Values is set to a placeholder so the
	// rest of the system stays happy).
	if pc.Attribute == schema.MDMCompliance {
		if err := validateMDMComplianceConfig(pc); err != nil {
			return err
		}
		pc.Values = datatypes.JSONSlice[string]{"mdm"}
		if len(pc.Tags) == 0 {
			pc.Tags = make(datatypes.JSONMap)
		}
		if len(pc.UserGroups) == 0 {
			pc.UserGroups = make(datatypes.JSONMap)
		}
		return nil
	}
	allowedAttrvaluesMap := schema.PostureCheckAttrValuesMap[pc.Attribute]
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
			_, err := GetUserGroup(schema.UserGroupID(userGrpID))
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

// MDMComplianceConfig is the typed view of PostureCheck.Config when
// Attribute == MDMCompliance.
type MDMComplianceConfig struct {
	RequireEnrolled  bool
	RequireCompliant bool
	MaxStateAgeHours int
}

// ParseMDMComplianceConfig decodes the JSONMap stored on PostureCheck.Config
// into a typed MDMComplianceConfig. Unknown keys are ignored.
func ParseMDMComplianceConfig(cfg datatypes.JSONMap) MDMComplianceConfig {
	out := MDMComplianceConfig{}
	if cfg == nil {
		return out
	}
	if v, ok := cfg["require_enrolled"]; ok {
		out.RequireEnrolled = asBool(v)
	}
	if v, ok := cfg["require_compliant"]; ok {
		out.RequireCompliant = asBool(v)
	}
	if v, ok := cfg["max_state_age_hours"]; ok {
		out.MaxStateAgeHours = asInt(v)
	}
	return out
}

func asBool(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true")
	case float64:
		return x != 0
	case int:
		return x != 0
	}
	return false
}

func asInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		if i, err := strconv.Atoi(x); err == nil {
			return i
		}
	}
	return 0
}

// emitNewMDMViolationEvents emits a posture_check_failed audit event for every
// MDM compliance violation that is newly present (not in oldVi) in newVi.
// Old violations don't re-fire; cleared violations are also ignored here.
func emitNewMDMViolationEvents(oldVi, newVi []models.Violation, d models.PostureCheckDeviceInfo, network schema.NetworkID) {
	if len(newVi) == 0 {
		return
	}
	prev := make(map[string]struct{}, len(oldVi))
	for _, v := range oldVi {
		prev[v.CheckID+"|"+v.Message] = struct{}{}
	}
	settings := logic.GetServerSettings()
	for _, v := range newVi {
		if v.Attribute != string(schema.MDMCompliance) {
			continue
		}
		if _, ok := prev[v.CheckID+"|"+v.Message]; ok {
			continue
		}
		diff := models.Diff{
			Old: nil,
			New: map[string]interface{}{
				"event":     "posture_check_failed",
				"type":      string(schema.MDMCompliance),
				"host_id":   d.HostID,
				"check_id":  v.CheckID,
				"check":     v.Name,
				"reason":    v.Message,
				"severity":  v.Severity,
				"provider":  settings.MDMProvider,
				"enrolled":  mdmStateEnrolled(d.MDMState),
				"compliant": mdmStateCompliant(d.MDMState),
			},
		}
		logic.LogEvent(&models.Event{
			Action: schema.PostureCheckFailed,
			Source: models.Subject{
				ID:   d.HostID,
				Name: d.HostID,
				Type: schema.DeviceSub,
			},
			TriggeredBy: "system",
			Target: models.Subject{
				ID:   v.CheckID,
				Name: v.Name,
				Type: schema.PostureCheckSub,
			},
			NetworkID: network,
			Origin:    schema.Api,
			Diff:      diff,
		})
	}
}

func mdmStateEnrolled(s *schema.DeviceMDMState) bool {
	if s == nil {
		return false
	}
	return s.Enrolled
}

func mdmStateCompliant(s *schema.DeviceMDMState) bool {
	if s == nil {
		return false
	}
	return s.Compliant
}

// validateMDMComplianceConfig enforces the MDMCompliance posture-check
// invariants: an MDM provider must be configured in ServerSettings, at least
// one of require_enrolled/require_compliant must be true, and
// max_state_age_hours must be non-negative.
func validateMDMComplianceConfig(pc *schema.PostureCheck) error {
	settings := logic.GetServerSettings()
	if settings.MDMProvider == "" {
		return errors.New("no MDM provider configured in Settings > Integrations > MDM")
	}
	cfg := ParseMDMComplianceConfig(pc.Config)
	if !cfg.RequireEnrolled && !cfg.RequireCompliant {
		return errors.New("at least one of require_enrolled or require_compliant must be true")
	}
	if cfg.MaxStateAgeHours < 0 {
		return errors.New("max_state_age_hours must be >= 0")
	}
	// Normalise the Config map so it's always present after validation.
	if pc.Config == nil {
		pc.Config = make(datatypes.JSONMap)
	}
	pc.Config["require_enrolled"] = cfg.RequireEnrolled
	pc.Config["require_compliant"] = cfg.RequireCompliant
	pc.Config["max_state_age_hours"] = cfg.MaxStateAgeHours
	return nil
}
