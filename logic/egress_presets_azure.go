package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
)

const azureServiceTagsDownloadID = "56519"

// azureServiceTagsConfirmURL is the Microsoft download confirmation page (overridable in tests).
var azureServiceTagsConfirmURL = fmt.Sprintf(
	"https://www.microsoft.com/en-us/download/confirmation.aspx?id=%s",
	azureServiceTagsDownloadID,
)

// azureServiceTagsJSONURLOverride, when non-empty, skips confirmation-page discovery (tests).
var azureServiceTagsJSONURLOverride string

var azureServiceTagsJSONURLPattern = regexp.MustCompile(
	`https://download\.microsoft\.com/download/[^"'\s>]+\.json`,
)

type azureServiceTagsDoc struct {
	Values []azureServiceTagEntry `json:"values"`
}

type azureServiceTagEntry struct {
	Name       string                    `json:"name"`
	Properties azureServiceTagProperties `json:"properties"`
}

type azureServiceTagProperties struct {
	Region          string   `json:"region"`
	AddressPrefixes []string `json:"addressPrefixes"`
}

func discoverAzureServiceTagsJSONURL(client *http.Client) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, azureServiceTagsConfirmURL, nil)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure service tags confirm page: status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	match := azureServiceTagsJSONURLPattern.FindString(string(body))
	if match == "" {
		return "", fmt.Errorf("azure service tags json url not found on confirm page")
	}
	return match, nil
}

func fetchAzureServiceTagsDoc(client *http.Client) (*azureServiceTagsDoc, error) {
	jsonURL := strings.TrimSpace(azureServiceTagsJSONURLOverride)
	if jsonURL == "" {
		var err error
		jsonURL, err = discoverAzureServiceTagsJSONURL(client)
		if err != nil {
			return nil, err
		}
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, jsonURL, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure service tags json: status %d", res.StatusCode)
	}
	var doc azureServiceTagsDoc
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func resolveAzurePresetCIDRs(client *http.Client, p models.EgressPresetApp) ([]string, error) {
	doc, err := fetchAzureServiceTagsDoc(client)
	if err != nil {
		return nil, err
	}
	var out []string
	switch {
	case p.ID == "azure-storage-global":
		for _, e := range doc.Values {
			if e.Name == "Storage" {
				out = append(out, e.Properties.AddressPrefixes...)
			}
		}
	case strings.HasPrefix(p.ID, "azure-storage-"):
		region := strings.TrimPrefix(p.ID, "azure-storage-")
		for _, e := range doc.Values {
			if !strings.HasPrefix(e.Name, "Storage.") {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(e.Properties.Region), region) {
				out = append(out, e.Properties.AddressPrefixes...)
			}
		}
	case strings.HasPrefix(p.ID, "azure-cloud-"):
		region := strings.TrimPrefix(p.ID, "azure-cloud-")
		for _, e := range doc.Values {
			if !strings.HasPrefix(e.Name, "AzureCloud.") {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(e.Properties.Region), region) {
				out = append(out, e.Properties.AddressPrefixes...)
			}
		}
	default:
		return nil, fmt.Errorf("unhandled Azure preset %q", p.ID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no CIDRs matched for Azure preset %q", p.ID)
	}
	return UniqueIPNetStrList(out), nil
}
