package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

type DatadogConfig struct {
	APIKey  string   `json:"api_key"`
	Site    string   `json:"site"`
	Service string   `json:"service"`
	Tags    []string `json:"tags"`
}

type datadogProvider struct{}

func (d *datadogProvider) Validate(configJSON json.RawMessage) error {
	var cfg DatadogConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid datadog config: %w", err)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}

func (d *datadogProvider) Test(configJSON json.RawMessage) error {
	var cfg DatadogConfig
	err := json.Unmarshal(configJSON, &cfg)
	if err != nil {
		return fmt.Errorf("invalid datadog config: %w", err)
	}

	testEvent := map[string]any{
		"message": "netmaker siem integration test",
	}
	return NewDatadogSIEMClient(cfg).Export(context.Background(), []any{testEvent})
}

type DatadogSIEMClient struct {
	DatadogConfig
}

func NewDatadogSIEMClient(config DatadogConfig) *DatadogSIEMClient {
	if config.Site == "" {
		config.Site = "datadoghq.com"
	}

	return &DatadogSIEMClient{
		DatadogConfig: config,
	}
}

func (d *DatadogSIEMClient) Export(ctx context.Context, events []any) error {
	ctx = context.WithValue(
		ctx,
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {Key: d.APIKey},
		},
	)
	ctx = context.WithValue(ctx, datadog.ContextServerVariables, map[string]string{
		"site": d.Site,
	})

	items := make([]datadogV2.HTTPLogItem, 0, len(events))
	for _, e := range events {
		msg, _ := json.Marshal(e)
		item := datadogV2.HTTPLogItem{
			Message:  string(msg),
			Ddsource: datadog.PtrString("netmaker"),
		}
		if d.Service != "" {
			item.Service = datadog.PtrString(d.Service)
		}
		if len(d.Tags) > 0 {
			item.Ddtags = datadog.PtrString(strings.Join(d.Tags, ","))
		}
		items = append(items, item)
	}

	apiClient := datadog.NewAPIClient(datadog.NewConfiguration())
	api := datadogV2.NewLogsApi(apiClient)
	_, _, err := api.SubmitLog(ctx, items, *datadogV2.NewSubmitLogOptionalParameters())
	if err != nil {
		return fmt.Errorf("failed to export to datadog: %w", err)
	}
	return nil
}
