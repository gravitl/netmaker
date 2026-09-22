package logic

import (
	"context"
	"encoding/json"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/grpc/siem"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/pro/integration"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"google.golang.org/protobuf/types/known/structpb"
)

func HandleExporterIntegrationPull(_ mqtt.Client, _ mqtt.Message) {
	tenants, err := (&schema.Tenant{}).List(db.WithContext(context.Background()))
	if err != nil {
		logger.Log(0, fmt.Sprintf("error listing tenants: %v", err))
		return
	}

	for _, tenant := range tenants {
		ctx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, tenant.ID)

		if !logic.GetFeatureFlags(ctx).EnableSIEMIntegration {
			if err := siem.Client().Terminate(ctx); err != nil {
				logger.Log(0, fmt.Sprintf("error terminating integration on exporter for tenant %s: %v", tenant.ID, err))
			}
			continue
		}

		intg := &schema.Integration{
			Type: string(integration.TypeSIEM),
		}
		integrations, err := intg.ListByType(ctx)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error checking integrations for tenant %s: %v", tenant.ID, err))
			continue
		}

		if len(integrations) > 1 {
			logger.Log(0, fmt.Sprintf("found more than one integration of type %s for tenant %s", intg.Type, tenant.ID))
			continue
		}

		if len(integrations) == 0 {
			err = siem.Client().Terminate(ctx)
			if err != nil {
				logger.Log(0, fmt.Sprintf("error terminating integration on exporter for tenant %s: %v", tenant.ID, err))
			}
			continue
		}

		configMap := make(map[string]interface{})
		err = json.Unmarshal(integrations[0].Config, &configMap)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error unmarshalling integration %s config for tenant %s: %v", integrations[0].Provider, tenant.ID, err))
			continue
		}

		configStruct, err := structpb.NewStruct(configMap)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error converting integration %s config for tenant %s: %v", integrations[0].Provider, tenant.ID, err))
			continue
		}

		err = siem.Client().Init(ctx, integrations[0].Provider, configStruct)
		if err != nil {
			logger.Log(0, fmt.Sprintf("error initializing integration %s on exporter for tenant %s: %v", integrations[0].Provider, tenant.ID, err))
		}
	}
}
