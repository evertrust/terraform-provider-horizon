//go:build e2e

// nolint
package tests

import (
	"context"
	"fmt"
	"net/url"

	horizon "github.com/evertrust/horizon-go/v2"
	"github.com/evertrust/horizon-go/v2/models"
	"golang.org/x/mod/semver"
)

func supportsWebRAChallenge(horizonVersion string) bool {
	version := "v" + horizonVersion
	if !semver.IsValid(version) {
		return false
	}
	return semver.Compare(semver.MajorMinor(version), "v2.11") >= 0
}

func issueWebRAChallenge(ctx context.Context, endpoint, profile string) (string, error) {
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	cfg := horizon.NewConfiguration()
	cfg.Servers = horizon.ServerConfigurations{{URL: endpoint}}
	cfg.Scheme = endpointURL.Scheme
	cfg.SetPasswordAuth(AdminUsername, AdminPassword)
	client := horizon.NewAPIClient(cfg)

	submit := models.NewWebRAEnrollRequestOnSubmit(profile, "webra", *models.NewWebRAEnrollRequestTemplateWithDefaults(), "enroll")
	resp, _, err := client.RequestAPI.RequestSubmit(ctx).
		RequestSubmitRequest(models.WebRAEnrollRequestOnSubmitAsRequestSubmitRequest(submit)).
		Execute()
	if err != nil {
		return "", fmt.Errorf("issuing a challenge on %s: %w", profile, err)
	}
	enroll := resp.WebRAEnrollRequestOnSubmitResponse
	if enroll == nil || enroll.Password.Get() == nil || enroll.Password.Get().GetValue() == "" {
		return "", fmt.Errorf("no challenge in the submit response on %s", profile)
	}
	return enroll.Password.Get().GetValue(), nil
}
