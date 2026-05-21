package logic

import (
	"context"
	"encoding/json"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/grpc/siem"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/pro/integration"
	"github.com/gravitl/netmaker/schema"
	"google.golang.org/protobuf/types/known/structpb"
)

func HandleExporterIntegrationPull(_ mqtt.Client, _ mqtt.Message) {
	if GetFeatureFlags().EnableSIEMIntegration {
		intg := &schema.Integration{
			Type: string(integration.TypeSIEM),
		}
		integrations, err := intg.ListByType(db.WithContext(context.TODO()))
		if err != nil {
			logger.Log(0, fmt.Sprintf("error checking integrations: %v", err))
			return
		}

		if len(integrations) > 1 {
			logger.Log(0, fmt.Sprintf("found more than one integration of type %s", intg.Type))
			return
		}

		if len(integrations) == 0 {
			err = siem.Client().Terminate(context.Background())
			if err != nil {
				logger.Log(0, fmt.Sprintf("error terminating integration on exporter: %v", err))
			}
		} else {
			configMap := make(map[string]interface{})
			err = json.Unmarshal(integrations[0].Config, &configMap)
			if err != nil {
				logger.Log(0, fmt.Sprintf("error unmarshalling integration %s config: %v", integrations[0].IntegrationID, err))
				return
			}

			configStruct, err := structpb.NewStruct(configMap)
			if err != nil {
				logger.Log(0, fmt.Sprintf("error converting integration %s config: %v", integrations[0].IntegrationID, err))
				return
			}

			err = siem.Client().Init(context.Background(), integrations[0].IntegrationID, configStruct)
			if err != nil {
				logger.Log(0, fmt.Sprintf("error initializing integration %s on exporter: %v", integrations[0].IntegrationID, err))
			}
		}
	} else {
		err := siem.Client().Terminate(context.Background())
		if err != nil {
			logger.Log(0, fmt.Sprintf("error terminating integration on exporter: %v", err))
		}
	}
}
