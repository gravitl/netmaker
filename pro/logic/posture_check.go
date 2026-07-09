package logic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/biter777/countries"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	mdmpkg "github.com/gravitl/netmaker/pro/integration/mdm"
	edrpkg "github.com/gravitl/netmaker/pro/integration/edr"
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
	// Refresh MDM/EDR before evaluating; bypass sync_enabled for the posture cycle.
	_ = mdmpkg.RunMDMSync(db.WithContext(context.TODO()))
	_ = edrpkg.RunEDRSyncForPosture(db.WithContext(context.TODO()))
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
					emitNewEDRViolationEvents(extclient.PostureChecksViolations, postureChecksViolations, deviceInfo, schema.NetworkID(netI.Name))
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
				emitNewEDRViolationEvents(nodeI.PostureChecksViolations, postureChecksViolations, deviceInfo, schema.NetworkID(netI.Name))

				_node := &schema.Node{
					ID:                                nodeI.ID.String(),
					PostureCheckSeverity:              postureCheckVolationSeverityLevel,
					PostureCheckLastEvaluationCycleID: uuid.NewString(),
					PostureCheckLastEvaluatedAt:       time.Now().UTC(),
				}

				_violations := make([]schema.PostureCheckViolation, 0, len(postureChecksViolations))
				for _, violation := range postureChecksViolations {
					_violations = append(_violations, schema.PostureCheckViolation{
						EvaluationCycleID: _node.PostureCheckLastEvaluationCycleID,
						CheckID:           violation.CheckID,
						NodeID:            _node.ID,
						Name:              violation.Name,
						Attribute:         violation.Attribute,
						Message:           violation.Message,
						Severity:          violation.Severity,
						EvaluatedAt:       _node.PostureCheckLastEvaluatedAt,
					})
				}
				_ = _node.UpsertViolations(db.WithContext(context.TODO()), _violations)
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

// CheckPostureViolationsForHost refreshes MDM/EDR snapshots for the host and
// evaluates network posture checks. Use this for registration and join flows
// instead of building PostureCheckDeviceInfo without integration state.
func CheckPostureViolationsForHost(
	host *schema.Host,
	tags map[models.TagID]struct{},
	network schema.NetworkID,
	skipAutoUpdate bool,
) ([]models.Violation, schema.Severity) {
	return CheckPostureViolations(GetPostureCheckDeviceInfoForHost(host, tags, skipAutoUpdate, true), network)
}

func GetPostureCheckDeviceInfoForHost(
	host *schema.Host,
	tags map[models.TagID]struct{},
	skipAutoUpdate bool,
	refreshIntegration bool,
) models.PostureCheckDeviceInfo {
	if host == nil {
		return models.PostureCheckDeviceInfo{}
	}
	deviceInfo := models.PostureCheckDeviceInfo{
		ClientLocation: host.CountryCode,
		ClientVersion:  host.Version,
		OS:             host.OS,
		OSVersion:      host.OSVersion,
		OSFamily:       host.OSFamily,
		KernelVersion:  host.KernelVersion,
		AutoUpdate:     host.AutoUpdate,
		SkipAutoUpdate: skipAutoUpdate,
		Tags:           tags,
		HostID:         host.ID.String(),
	}
	ctx := db.WithContext(context.TODO())
	if refreshIntegration {
		_ = mdmpkg.RefreshHostMDMState(ctx, *host)
		_ = edrpkg.RefreshHostEDRState(ctx, *host)
	}
	attachIntegrationStates(ctx, host.ID.String(), &deviceInfo)
	return deviceInfo
}

func attachIntegrationStates(ctx context.Context, hostID string, d *models.PostureCheckDeviceInfo) {
	if d == nil || hostID == "" {
		return
	}
	d.HostID = hostID
	if providerID, err := mdmpkg.ActiveProviderID(ctx); err == nil && providerID != "" {
		state := &schema.DeviceMDMState{HostID: hostID, Provider: providerID}
		if err := state.Get(ctx); err == nil {
			d.MDMState = state
		}
	}
	if edrProviderID, err := edrpkg.ActiveProviderID(ctx); err == nil && edrProviderID != "" {
		edrState := &schema.DeviceEDRState{HostID: hostID, Provider: edrProviderID}
		if err := edrState.Get(ctx); err == nil {
			d.EDRState = edrState
		}
	}
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
			} else if (c.Attribute == schema.MDMCompliance || c.Attribute == schema.EDRCompliance) && len(c.Tags) == 0 {
				// Legacy MDM/EDR checks saved before wildcard default; apply to all hosts.
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
		ctx := db.WithContext(context.TODO())
		_ = mdmpkg.RefreshHostMDMState(ctx, *h)
		_ = edrpkg.RefreshHostEDRState(ctx, *h)
		attachIntegrationStates(ctx, h.ID.String(), &deviceInfo)
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
				if _, ok := user.UserGroups.Data()[GetDefaultGlobalAdminGroupID()]; ok {

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
			return true, "No MDM status found for this device. It may not be enrolled or matched yet."
		}
		if d.MDMState.LastError != "" {
			slog.Warn("mdm state error during posture check", "host_id", d.HostID, "error", d.MDMState.LastError)
			return true, "Unable to verify MDM status for this device."
		}
		if cfg.RequireEnrolled && !d.MDMState.Enrolled {
			return true, "Device is not enrolled in MDM."
		}
		if cfg.RequireCompliant {
			providerID, _ := mdmpkg.ActiveProviderID(db.WithContext(context.TODO()))
			if mdmpkg.CapabilitiesFor(providerID).ReportsCompliant && !d.MDMState.Compliant {
				return true, "Device is not MDM compliant."
			}
		}
		if cfg.MaxStateAgeHours > 0 &&
			time.Since(d.MDMState.LastSyncedAt) > time.Duration(cfg.MaxStateAgeHours)*time.Hour {
			return true, "MDM status is outdated. Re-sync the device and try again."
		}

	// ------------------------
	// 7. EDR compliance check
	// Config: {require_agent_installed, require_agent_healthy,
	//          max_allowed_risk_level, max_state_age_hours}
	// ------------------------
	case schema.EDRCompliance:
		cfg := ParseEDRComplianceConfig(check.Config)
		if d.EDRState == nil {
			return true, "No EDR status found for this device. It may not be protected or matched yet."
		}
		if d.EDRState.LastError != "" {
			slog.Warn("edr state error during posture check", "host_id", d.HostID, "error", d.EDRState.LastError)
			return true, "Unable to verify EDR status for this device."
		}
		if cfg.RequireAgentInstalled && !d.EDRState.AgentInstalled {
			return true, "EDR agent is not installed on this device."
		}
		if cfg.RequireAgentHealthy && !d.EDRState.AgentHealthy {
			return true, "EDR agent is not healthy on this device."
		}
		if cfg.MaxAllowedRiskLevel != "" {
			actual := edrpkg.ParseRiskLevel(d.EDRState.RiskLevel)
			if actual == edrpkg.RiskUnknown {
				return true, "EDR risk level could not be determined."
			}
			maxAllowed := edrpkg.ParseRiskLevel(cfg.MaxAllowedRiskLevel)
			if edrpkg.RiskExceeds(maxAllowed, actual) {
				return true, "EDR risk level exceeds the allowed maximum."
			}
		}
		if cfg.MaxStateAgeHours > 0 &&
			time.Since(d.EDRState.LastSyncedAt) > time.Duration(cfg.MaxStateAgeHours)*time.Hour {
			return true, "EDR status is outdated. Re-sync the device and try again."
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

// MergePostureCheckUpdate fills in fields omitted from an update payload using
// the existing stored posture check. Clients that toggle status often omit
// attribute-specific Config; without this merge validation would see empty
// MDM flags and reject the request.
func MergePostureCheckUpdate(existing, update *schema.PostureCheck) {
	if update.Attribute == schema.MDMCompliance {
		existingCfg := existing.Config
		if existingCfg == nil {
			existingCfg = defaultMDMComplianceConfig()
		}
		mergePostureCheckConfig(&schema.PostureCheck{Config: existingCfg}, update)
		return
	}
	if update.Attribute == schema.EDRCompliance {
		existingCfg := existing.Config
		if existingCfg == nil {
			existingCfg = defaultEDRComplianceConfig()
		}
		mergePostureCheckConfig(&schema.PostureCheck{Config: existingCfg}, update)
	}
}

func defaultMDMComplianceConfig() datatypes.JSONMap {
	return datatypes.JSONMap{
		"require_enrolled":    true,
		"require_compliant":   false,
		"max_state_age_hours": 0,
	}
}

func defaultEDRComplianceConfig() datatypes.JSONMap {
	return datatypes.JSONMap{
		"require_agent_installed": true,
		"require_agent_healthy":   false,
		"max_allowed_risk_level":  "",
		"max_state_age_hours":     0,
	}
}

func mergePostureCheckConfig(existing, update *schema.PostureCheck) {
	if update.Config == nil {
		update.Config = existing.Config
		return
	}
	merged := datatypes.JSONMap{}
	for k, v := range existing.Config {
		merged[k] = v
	}
	for k, v := range update.Config {
		merged[k] = v
	}
	update.Config = merged
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
			pc.Tags = datatypes.JSONMap{"*": "*"}
		}
		if len(pc.UserGroups) == 0 {
			pc.UserGroups = make(datatypes.JSONMap)
		}
		return nil
	}
	if pc.Attribute == schema.EDRCompliance {
		if err := validateEDRComplianceConfig(pc); err != nil {
			return err
		}
		pc.Values = datatypes.JSONSlice[string]{"edr"}
		if len(pc.Tags) == 0 {
			pc.Tags = datatypes.JSONMap{"*": "*"}
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
	providerID, _ := mdmpkg.ActiveProviderID(db.WithContext(context.TODO()))
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
				"provider":  providerID,
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

// emitNewEDRViolationEvents emits a posture_check_failed audit event for every
// EDR compliance violation that is newly present (not in oldVi) in newVi.
// Old violations don't re-fire; cleared violations are also ignored here.
func emitNewEDRViolationEvents(oldVi, newVi []models.Violation, d models.PostureCheckDeviceInfo, network schema.NetworkID) {
	if len(newVi) == 0 {
		return
	}
	prev := make(map[string]struct{}, len(oldVi))
	for _, v := range oldVi {
		prev[v.CheckID+"|"+v.Message] = struct{}{}
	}
	providerID, _ := edrpkg.ActiveProviderID(db.WithContext(context.TODO()))
	for _, v := range newVi {
		if v.Attribute != string(schema.EDRCompliance) {
			continue
		}
		if _, ok := prev[v.CheckID+"|"+v.Message]; ok {
			continue
		}
		diff := models.Diff{
			Old: nil,
			New: map[string]interface{}{
				"event":            "posture_check_failed",
				"type":             string(schema.EDRCompliance),
				"host_id":          d.HostID,
				"check_id":         v.CheckID,
				"check":            v.Name,
				"reason":           v.Message,
				"severity":         v.Severity,
				"provider":         providerID,
				"agent_installed":  edrStateAgentInstalled(d.EDRState),
				"agent_healthy":    edrStateAgentHealthy(d.EDRState),
				"risk_level":       edrStateRiskLevel(d.EDRState),
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

func edrStateAgentInstalled(s *schema.DeviceEDRState) bool {
	if s == nil {
		return false
	}
	return s.AgentInstalled
}

func edrStateAgentHealthy(s *schema.DeviceEDRState) bool {
	if s == nil {
		return false
	}
	return s.AgentHealthy
}

func edrStateRiskLevel(s *schema.DeviceEDRState) string {
	if s == nil {
		return ""
	}
	return s.RiskLevel
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
// invariants: an MDM integration must be configured, at least one of
// require_enrolled/require_compliant must be true when the check is enabled,
// and max_state_age_hours must be non-negative.
func validateMDMComplianceConfig(pc *schema.PostureCheck) error {
	active, err := mdmpkg.GetActive(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	if active == nil {
		return errors.New("no MDM integration configured; configure via Integrations > MDM")
	}
	cfg := ParseMDMComplianceConfig(pc.Config)
	if pc.Status && !cfg.RequireEnrolled && !cfg.RequireCompliant {
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

// EDRComplianceConfig is the typed view of PostureCheck.Config when
// Attribute == EDRCompliance.
type EDRComplianceConfig struct {
	RequireAgentInstalled bool
	RequireAgentHealthy   bool
	MaxAllowedRiskLevel   string
	MaxStateAgeHours      int
}

func ParseEDRComplianceConfig(cfg datatypes.JSONMap) EDRComplianceConfig {
	out := EDRComplianceConfig{}
	if cfg == nil {
		return out
	}
	if v, ok := cfg["require_agent_installed"]; ok {
		out.RequireAgentInstalled = asBool(v)
	}
	if v, ok := cfg["require_agent_healthy"]; ok {
		out.RequireAgentHealthy = asBool(v)
	}
	if v, ok := cfg["max_allowed_risk_level"]; ok {
		if s, ok := v.(string); ok {
			out.MaxAllowedRiskLevel = s
		}
	}
	if v, ok := cfg["max_state_age_hours"]; ok {
		out.MaxStateAgeHours = asInt(v)
	}
	return out
}

func validateEDRComplianceConfig(pc *schema.PostureCheck) error {
	active, err := edrpkg.GetActive(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	if active == nil {
		return errors.New("no EDR integration configured; configure via Integrations > EDR")
	}
	cfg := ParseEDRComplianceConfig(pc.Config)
	if pc.Status && !cfg.RequireAgentInstalled && !cfg.RequireAgentHealthy &&
		cfg.MaxAllowedRiskLevel == "" && cfg.MaxStateAgeHours == 0 {
		return errors.New("at least one EDR policy requirement must be set")
	}
	if cfg.MaxStateAgeHours < 0 {
		return errors.New("max_state_age_hours must be >= 0")
	}
	if pc.Config == nil {
		pc.Config = make(datatypes.JSONMap)
	}
	pc.Config["require_agent_installed"] = cfg.RequireAgentInstalled
	pc.Config["require_agent_healthy"] = cfg.RequireAgentHealthy
	if cfg.MaxAllowedRiskLevel != "" {
		pc.Config["max_allowed_risk_level"] = cfg.MaxAllowedRiskLevel
	}
	pc.Config["max_state_age_hours"] = cfg.MaxStateAgeHours
	return nil
}
