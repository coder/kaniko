/*
Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Ported with modifications from https://github.com/chrismellard/docker-credential-acr-env/blob/82a0ddb27589/pkg/credhelper/helper.go
// (Copyright 2020 Chris Mellard, Apache License 2.0) to fix GO-2026-6225:
// upstream matches registry hosts with an unanchored regular expression, so
// hosts such as evil.azurecr.io.attacker.com received Azure credentials.
// Upstream has no fix. Modifications: the hostname check below.

// Package acr is a Docker credential helper for Azure Container Registry that
// exchanges Azure credentials from the environment for a registry token.
package acr

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/docker/docker-credential-helpers/credentials"
)

// acrRE matches Azure Container Registry hostnames: one or more DNS labels
// followed by an ACR domain. It is anchored at both ends, unlike upstream.
var acrRE = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.)+azurecr\.(?:io|cn|de|us)$`)

const (
	mcrHostname   = "mcr.microsoft.com"
	tokenUsername = "<token>"
)

// ACRCredHelper implements credentials.Helper for Azure Container Registry.
type ACRCredHelper struct{}

// NewACRCredentialsHelper returns a credential helper for Azure Container
// Registry.
func NewACRCredentialsHelper() credentials.Helper {
	return &ACRCredHelper{}
}

// Add is not implemented.
func (a ACRCredHelper) Add(_ *credentials.Credentials) error {
	return errors.New("list is unimplemented")
}

// Delete is not implemented.
func (a ACRCredHelper) Delete(_ string) error {
	return errors.New("list is unimplemented")
}

func isACRRegistry(input string) bool {
	serverURL, err := url.Parse("https://" + input)
	if err != nil {
		return false
	}
	// A trailing dot denotes the same fully qualified host.
	host := strings.TrimSuffix(serverURL.Hostname(), ".")
	if host == mcrHostname {
		return true
	}
	return acrRE.MatchString(host)
}

// Get returns a registry refresh token for serverURL if it is an Azure
// Container Registry host.
func (a ACRCredHelper) Get(serverURL string) (string, string, error) {
	if !isACRRegistry(serverURL) {
		return "", "", errors.New("serverURL does not refer to Azure Container Registry")
	}

	spToken, settings, err := GetServicePrincipalTokenFromEnvironment()
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire sp token %w", err)
	}
	refreshToken, err := GetRegistryRefreshTokenFromAADExchange(serverURL, spToken, settings.Values[auth.TenantID])
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire refresh token %w", err)
	}
	return tokenUsername, refreshToken, nil
}

// List is not implemented.
func (a ACRCredHelper) List() (map[string]string, error) {
	return nil, errors.New("list is unimplemented")
}
