package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
)

// errHostAuthRegistrationUnsupported is returned to clients that attempt to register a host
// via SSO or basic auth (netclient join/register -s/-u). Only enrollment tokens are supported.
var errHostAuthRegistrationUnsupported = errors.New("registering a host via SSO/basic auth is no longer supported; use an enrollment token: netclient join -t <token>")

// SessionHandler - called by the HTTP router when a client dials in to register a host
// via SSO/basic auth. That flow is no longer supported, so the session is rejected
// and the reason is sent to the client in the websocket close message.
func SessionHandler(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()
	handleHostRegErr(conn, errHostAuthRegistrationUnsupported)
}

func init() {
	logic.JoinHostToNetworks = joinHostToNetworks
}

func joinHostToNetworks(ctx context.Context, key models.EnrollmentKey, host *schema.Host, username string) {
	featureFlags := logic.GetFeatureFlags(ctx)
	keyTags := make(map[models.TagID]struct{})
	if len(key.Groups) > 0 {
		for _, tagI := range key.Groups {
			keyTags[tagI] = struct{}{}
		}
	} else {
		for _, tagI := range key.Tags {
			keyTags[models.TagID(tagI)] = struct{}{}
		}
	}
	schemaKey := logic.SchemaEnrollmentKeyFromModels(key)
	for _, netID := range key.Networks {
		network := &schema.Network{Name: netID}
		if err := network.Get(ctx); err != nil {
			continue
		}
		if logic.DoesHostExistInTheNetworkAlready(host, network) {
			continue
		}

		violations, _ := logic.CheckPostureViolationsForHost(
			ctx,
			host,
			keyTags,
			schema.NetworkID(network.Name),
			true,
		)
		if len(violations) > 0 {
			logic.EmitNewPostureViolationEvents(ctx, nil, violations, models.PostureCheckDeviceInfo{HostID: host.ID.String()}, schema.NetworkID(network.Name))
			logger.Log(0, fmt.Sprintf("skipping joining network %s due to violations", network.Name))
			continue
		}

		if featureFlags.EnableDeviceApproval && !network.AutoJoin && !key.SkipDeviceApproval {
			if err := (&schema.PendingHost{
				HostID:  host.ID.String(),
				Network: netID,
			}).CheckIfPendingHostExists(ctx); err == nil {
				continue
			}
			keyB, _ := json.Marshal(key)
			p := schema.PendingHost{
				ID:            uuid.NewString(),
				TenantID:      scope.ID(ctx),
				HostID:        host.ID.String(),
				Hostname:      host.Name,
				Network:       netID,
				PublicKey:     host.PublicKey.String(),
				OS:            host.OS,
				Location:      host.Location,
				Version:       host.Version,
				EnrollmentKey: keyB,
				RequestedAt:   time.Now().UTC(),
			}
			_ = p.Create(ctx)
			continue
		}

		_, err := orchestrator.GetRepository().NodeOrchestrator().CreateNode(
			ctx,
			host,
			network,
			orchestrator.UseKey(schemaKey),
			orchestrator.SkipHostUpdate(),
			orchestrator.SkipPublishPeerUpdate(),
		)
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed to add host (%s, %s) to network (%s): %v", host.ID.String(), host.Name, netID, err.Error()))
			continue
		}

		if len(username) > 0 {
			logic.LogEvent(ctx, &models.Event{
				Action: schema.JoinHostToNet,
				Source: models.Subject{
					ID:   username,
					Name: username,
					Type: schema.UserSub,
				},
				TriggeredBy: username,
				Target: models.Subject{
					ID:   host.ID.String(),
					Name: host.Name,
					Type: schema.DeviceSub,
				},
				NetworkID: schema.NetworkID(netID),
				Origin:    schema.Dashboard,
			})
		} else if len(key.Tags) > 0 || len(key.Groups) > 0 {
			sourceName := ""
			if len(key.Groups) > 0 {
				sourceName = key.Groups[0].String()
			} else if len(key.Tags) > 0 {
				sourceName = key.Tags[0]
			}
			if sourceName == "" {
				sourceName = schemaKey.Name
			}
			logic.LogEvent(ctx, &models.Event{
				Action: schema.JoinHostToNet,
				Source: models.Subject{
					ID:   key.Value,
					Name: sourceName,
					Type: schema.EnrollmentKeySub,
				},
				TriggeredBy: username,
				Target: models.Subject{
					ID:   host.ID.String(),
					Name: host.Name,
					Type: schema.DeviceSub,
				},
				NetworkID: schema.NetworkID(netID),
				Origin:    schema.Dashboard,
			})
		}
	}
	_ = logic.PublishHostRegistrationUpdates(ctx, host)
	if servercfg.IsMessageQueueBackend() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.RequestAck,
			Host:   *host,
		})
		if err := mq.PublishPeerUpdate(ctx, false); err != nil {
			logger.Log(0, "failed to publish peer update during registration -", err.Error())
		}
	}
}

// handleHostRegErr sends a close message on the websocket connection
// along with the error reason. Clients can assume an error close when
// the text is non-empty and a successful close when the text is empty.
func handleHostRegErr(conn *websocket.Conn, err error) {
	logger.Log(0, "error during host registration via auth:", err.Error())
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()))
	if err != nil {
		logger.Log(0, "error during message writing:", err.Error())
	}
}
