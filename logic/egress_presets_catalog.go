package logic

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// awsEgressRegionCodes is the public AWS region set covered by the preset catalog.
var awsEgressRegionCodes = []string{
	"af-south-1", "ap-east-1", "ap-east-2", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ap-south-1", "ap-south-2", "ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-southeast-4",
	"ap-southeast-5", "ap-southeast-6", "ap-southeast-7", "ca-central-1", "ca-west-1", "cn-north-1",
	"cn-northwest-1", "eu-central-1", "eu-central-2", "eusc-de-east-1", "eu-north-1", "eu-south-1", "eu-south-2",
	"eu-west-1", "eu-west-2", "eu-west-3", "il-central-1", "me-central-1", "me-south-1", "me-west-1",
	"mx-central-1", "sa-east-1", "sa-west-1", "us-east-1", "us-east-2", "us-gov-east-1", "us-gov-west-1",
	"us-west-1", "us-west-2",
}

// awsIPRangesURL is the public AWS IP ranges document (overridable in tests).
var awsIPRangesURL = "https://ip-ranges.amazonaws.com/ip-ranges.json"

//go:embed egress_preset_extras.json
var egressPresetExtrasJSON []byte

func buildEgressPresetCatalog() []models.EgressPresetApp {
	var out []models.EgressPresetApp
	for _, r := range awsEgressRegionCodes {
		out = append(out, awsS3Preset(r), awsEC2ELBPreset(r))
	}
	out = append(out, awsS3Global(), awsCloudFrontGlobal())

	var extras []models.EgressPresetApp
	if err := json.Unmarshal(egressPresetExtrasJSON, &extras); err != nil {
		logger.Log(0, "egress preset extras unmarshal: ", err.Error())
	} else {
		for i := range extras {
			trimEgressPresetDomains(&extras[i])
		}
		out = append(out, extras...)
	}
	return out
}

func awsS3Preset(region string) models.EgressPresetApp {
	return models.EgressPresetApp{
		Name:    fmt.Sprintf("AWS S3 (%s)", region),
		ID:      "aws-s3-" + region,
		Sources: []string{awsIPRangesURL},
		Domains: []string{
			"*.s3-website-" + region + ".amazonaws.com",
			"*.s3." + region + ".amazonaws.com",
			"s3-website-" + region + ".amazonaws.com",
			"s3." + region + ".amazonaws.com",
		},
		Group:           "aws",
		SuggestedDomain: "s3." + region + ".amazonaws.com",
	}
}

func awsEC2ELBPreset(region string) models.EgressPresetApp {
	return models.EgressPresetApp{
		Name:    fmt.Sprintf("AWS EC2/ELB (%s)", region),
		ID:      "aws-ec2-" + region,
		Sources: []string{awsIPRangesURL},
		Domains: []string{
			"*." + region + ".compute.amazonaws.com",
			"*." + region + ".elb.amazonaws.com",
			region + ".compute.amazonaws.com",
			region + ".elb.amazonaws.com",
		},
		Group:           "aws",
		SuggestedDomain: region + ".compute.amazonaws.com",
	}
}

func awsS3Global() models.EgressPresetApp {
	return models.EgressPresetApp{
		Name:    "AWS S3 (global)",
		ID:      "aws-s3-global",
		Sources: []string{awsIPRangesURL},
		Domains: []string{
			"*.s3-accelerate.amazonaws.com",
			"*.s3-accelerate.dualstack.amazonaws.com",
			"*.s3.amazonaws.com",
			"s3-accelerate.amazonaws.com",
			"s3-accelerate.dualstack.amazonaws.com",
			"s3.amazonaws.com",
		},
		Group:           "aws",
		SuggestedDomain: "s3.amazonaws.com",
	}
}

func awsCloudFrontGlobal() models.EgressPresetApp {
	return models.EgressPresetApp{
		Name:    "AWS CloudFront (global)",
		ID:      "aws-cloudfront-global",
		Sources: []string{awsIPRangesURL},
		Domains: []string{
			"*.cloudfront.net",
			"cloudfront.net",
		},
		Group:           "aws",
		SuggestedDomain: "cloudfront.net",
	}
}
