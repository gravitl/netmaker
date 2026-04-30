package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func migrateV1_5_1(ctx context.Context) error {
	err := migrateUsers(ctx)
	if err != nil {
		return err
	}

	err = migrateNetworks(ctx)
	if err != nil {
		return err
	}

	err = migrateUserRoles(ctx)
	if err != nil {
		return err
	}

	err = migrateUserGroups(ctx)
	if err != nil {
		return err
	}

	return migrateHosts(ctx)
}

func migrateUsers(ctx context.Context) error {
	records, err := FetchAll(ctx, database.USERS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var user models.User
		err = json.Unmarshal([]byte(record), &user)
		if err != nil {
			return err
		}

		platformRoleID := user.PlatformRoleID
		if user.PlatformRoleID == "" {
			if user.IsSuperAdmin {
				platformRoleID = schema.SuperAdminRole
			} else if user.IsAdmin {
				platformRoleID = schema.AdminRole
			} else {
				platformRoleID = schema.ServiceUser
			}
		}

		_user := &schema.User{
			ID:                         "",
			Username:                   user.UserName,
			DisplayName:                user.DisplayName,
			PlatformRoleID:             platformRoleID,
			ExternalIdentityProviderID: user.ExternalIdentityProviderID,
			AccountDisabled:            user.AccountDisabled,
			AuthType:                   user.AuthType,
			Password:                   user.Password,
			IsMFAEnabled:               user.IsMFAEnabled,
			TOTPSecret:                 user.TOTPSecret,
			LastLoginAt:                user.LastLoginTime,
			UserGroups:                 datatypes.NewJSONType(user.UserGroups),
			CreatedBy:                  user.CreatedBy,
			CreatedAt:                  user.CreatedAt,
			UpdatedAt:                  user.UpdatedAt,
		}

		logger.Log(4, fmt.Sprintf("migrating user %s", _user.Username))

		err = _user.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user %s failed: %v", _user.Username, err))
			return err
		}
	}

	return nil
}

func migrateNetworks(ctx context.Context) error {
	records, err := FetchAll(ctx, database.NETWORKS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var network models.Network
		err = json.Unmarshal([]byte(record), &network)
		if err != nil {
			return err
		}

		var autoJoin, autoRemove, jitEnabled bool

		if network.AutoJoin == "false" {
			autoJoin = false
		} else {
			autoJoin = true
		}

		if network.AutoRemove == "true" {
			autoRemove = true
		} else {
			autoRemove = false
		}

		if network.JITEnabled == "yes" {
			jitEnabled = true
		} else {
			jitEnabled = false
		}

		_network := &schema.Network{
			ID:                          "",
			Name:                        network.NetID,
			AddressRange:                network.AddressRange,
			AddressRange6:               network.AddressRange6,
			DefaultKeepAlive:            int(network.DefaultKeepalive),
			DefaultMTU:                  network.DefaultMTU,
			AutoJoin:                    autoJoin,
			AutoRemove:                  autoRemove,
			AutoRemoveTags:              network.AutoRemoveTags,
			AutoRemoveThreshold:         network.AutoRemoveThreshold,
			JITEnabled:                  jitEnabled,
			VirtualNATPoolIPv4:          network.VirtualNATPoolIPv4,
			VirtualNATSitePrefixLenIPv4: network.VirtualNATSitePrefixLenIPv4,
			NodesUpdatedAt:              time.Unix(network.NodesLastModified, 0),
			CreatedBy:                   network.CreatedBy,
			CreatedAt:                   network.CreatedAt,
			UpdatedAt:                   time.Unix(network.NetworkLastModified, 0),
		}

		logger.Log(4, fmt.Sprintf("migrating network %s", _network.Name))

		err = _network.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating network %s failed: %v", _network.Name, err))
			return err
		}

		var cidr, cidrv6 *net.IPNet
		if len(network.AddressRange) != 0 {
			_, cidr, err = net.ParseCIDR(network.AddressRange)
			if err != nil {
				err = fmt.Errorf("error parsing network (%s) cidr (%s): %v", _network.Name, network.AddressRange, err)
				logger.Log(4, fmt.Sprintf("migrating network %s failed: %v", _network.Name, err))
				return err
			}
		}

		if len(network.AddressRange6) != 0 {
			_, cidrv6, err = net.ParseCIDR(network.AddressRange6)
			if err != nil {
				err = fmt.Errorf("error parsing network (%s) cidr (%s): %v", _network.Name, network.AddressRange6, err)
				logger.Log(4, fmt.Sprintf("migrating network %s failed: %v", _network.Name, err))
				return err
			}
		}

		superAdmin := &schema.User{}
		err = superAdmin.GetSuperAdmin(ctx)
		if err != nil {
			err = fmt.Errorf("error getting superadmin: %v", err)
			logger.Log(4, fmt.Sprintf("migrating network %s failed: %v", _network.Name, err))
			return err
		}

		if len(network.NameServers) > 0 {
			ns := schema.Nameserver{
				ID:        uuid.NewString(),
				Name:      "upstream nameservers",
				NetworkID: _network.Name,
				Servers:   []string{},
				MatchAll:  true,
				Domains: []schema.NameserverDomain{
					{
						Domain: ".",
					},
				},
				Tags: datatypes.JSONMap{
					"*": struct{}{},
				},
				Nodes:     make(datatypes.JSONMap),
				Status:    true,
				CreatedBy: superAdmin.Username,
			}

			for _, nsIP := range network.NameServers {
				ip := net.ParseIP(nsIP)
				if ip == nil {
					continue
				}

				if ip.To4() != nil {
					if cidr != nil && !cidr.Contains(ip) {
						ns.Servers = append(ns.Servers, nsIP)
					}
				} else {
					if cidrv6 != nil && !cidrv6.Contains(ip) {
						ns.Servers = append(ns.Servers, nsIP)
					}
				}
			}

			if len(ns.Servers) > 0 {
				err = ns.Create(ctx)
				if err != nil {
					err = fmt.Errorf("error creating upstream nameserver for network (%s): %v", _network.Name, err)
					logger.Log(4, fmt.Sprintf("migrating network %s failed: %v", _network.Name, err))
					return err
				}
			}
		}
	}

	return nil
}

func migrateUserRoles(ctx context.Context) error {
	records, err := FetchAll(ctx, database.USER_PERMISSIONS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var _userRole schema.UserRole
		err = json.Unmarshal([]byte(record), &_userRole)
		if err != nil {
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating user role %s", _userRole.ID))

		err = _userRole.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user role %s failed: %v", _userRole.ID, err))
			return err
		}
	}

	return nil
}

func migrateUserGroups(ctx context.Context) error {
	records, err := FetchAll(ctx, database.USER_GROUPS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var _userGroup schema.UserGroup
		err = json.Unmarshal([]byte(record), &_userGroup)
		if err != nil {
			return err
		}

		logger.Log(4, fmt.Sprintf("migrating user group %s", _userGroup.ID))

		err = _userGroup.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating user group %s failed: %v", _userGroup.ID, err))
			return err
		}
	}

	return nil
}

func migrateHosts(ctx context.Context) error {
	records, err := FetchAll(ctx, database.HOSTS_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}

	for _, record := range records {
		var host models.Host
		err = json.Unmarshal([]byte(record), &host)
		if err != nil {
			return err
		}

		_host := &schema.Host{
			ID:                 host.ID,
			Verbosity:          host.Verbosity,
			FirewallInUse:      host.FirewallInUse,
			Version:            host.Version,
			IPForwarding:       host.IPForwarding,
			DaemonInstalled:    host.DaemonInstalled,
			AutoUpdate:         host.AutoUpdate,
			HostPass:           host.HostPass,
			Name:               host.Name,
			OS:                 host.OS,
			OSFamily:           host.OSFamily,
			OSVersion:          host.OSVersion,
			KernelVersion:      host.KernelVersion,
			Interface:          host.Interface,
			Debug:              host.Debug,
			ListenPort:         host.ListenPort,
			WgPublicListenPort: host.WgPublicListenPort,
			MTU:                host.MTU,
			PublicKey: schema.WgKey{
				Key: host.PublicKey,
			},
			MacAddress:          host.MacAddress,
			TrafficKeyPublic:    host.TrafficKeyPublic,
			Nodes:               host.Nodes,
			Interfaces:          host.Interfaces,
			DefaultInterface:    host.DefaultInterface,
			EndpointIP:          host.EndpointIP,
			EndpointIPv6:        host.EndpointIPv6,
			IsDocker:            host.IsDocker,
			IsK8S:               host.IsK8S,
			IsStaticPort:        host.IsStaticPort,
			IsStatic:            host.IsStatic,
			IsDefault:           host.IsDefault,
			DNS:                 host.DNS,
			NatType:             host.NatType,
			TurnEndpoint:        nil,
			PersistentKeepalive: host.PersistentKeepalive,
			Location:            host.Location,
			CountryCode:         host.CountryCode,
			EnableFlowLogs:      host.EnableFlowLogs,
		}

		if host.TurnEndpoint != nil {
			_host.TurnEndpoint = &schema.AddrPort{
				AddrPort: *host.TurnEndpoint,
			}
		}

		if _host.PersistentKeepalive == 0 {
			_host.PersistentKeepalive = models.DefaultPersistentKeepAlive
		}

		if _host.DNS == "" || (_host.DNS != "yes" && _host.DNS != "no") {
			if logic.GetServerSettings().ManageDNS {
				_host.DNS = "yes"
			} else {
				_host.DNS = "no"
			}
			if _host.IsDefault {
				_host.DNS = "yes"
			}
		}

		logger.Log(4, fmt.Sprintf("migrating host %s", _host.ID))

		err = _host.Create(ctx)
		if err != nil {
			logger.Log(4, fmt.Sprintf("migrating host %s failed: %v", _host.ID, err))
			return err
		}
	}

	return nil
}

func FetchAll(ctx context.Context, tableName string) (map[string]string, error) {
	row, err := db.FromContext(ctx).Raw("SELECT * FROM " + tableName + " ORDER BY key").Rows()
	if err != nil {
		return nil, err
	}
	records := make(map[string]string)
	defer row.Close()
	for row.Next() { // Iterate and fetch the records from result cursor
		var key string
		var value string
		row.Scan(&key, &value)
		records[key] = value
	}
	if len(records) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return records, nil
}
