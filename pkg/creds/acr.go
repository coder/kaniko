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

// Ported from https://github.com/chrismellard/docker-credential-acr-env/blob/82a0ddb27589/pkg/credhelper/helper.go
// (Copyright 2020 Chris Mellard, Apache License 2.0) to fix GO-2026-6225: the
// upstream registry hostname check is unanchored, so hosts such as
// evil.azurecr.io.attacker.com received Azure credentials. Upstream has no fix.

package creds

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/chrismellard/docker-credential-acr-env/pkg/registry"
	"github.com/chrismellard/docker-credential-acr-env/pkg/token"
	"github.com/docker/docker-credential-helpers/credentials"
)

// acrRE matches Azure Container Registry hostnames. Unlike upstream, it is
// anchored so that only hosts ending in an ACR domain match.
var acrRE = regexp.MustCompile(`^(?:.*\.azurecr\.io|.*\.azurecr\.cn|.*\.azurecr\.de|.*\.azurecr\.us)$`)

const (
	mcrHostname      = "mcr.microsoft.com"
	acrTokenUsername = "<token>"
)

// acrCredHelper exchanges Azure service principal credentials from the
// environment for an Azure Container Registry refresh token.
type acrCredHelper struct{}

func newACRCredentialsHelper() credentials.Helper {
	return acrCredHelper{}
}

func (acrCredHelper) Add(_ *credentials.Credentials) error {
	return errors.New("add is unimplemented")
}

func (acrCredHelper) Delete(_ string) error {
	return errors.New("delete is unimplemented")
}

func (acrCredHelper) List() (map[string]string, error) {
	return nil, errors.New("list is unimplemented")
}

func isACRRegistry(input string) bool {
	serverURL, err := url.Parse("https://" + input)
	if err != nil {
		return false
	}
	if serverURL.Hostname() == mcrHostname {
		return true
	}
	return acrRE.MatchString(serverURL.Hostname())
}

func (acrCredHelper) Get(serverURL string) (string, string, error) {
	if !isACRRegistry(serverURL) {
		return "", "", errors.New("serverURL does not refer to Azure Container Registry")
	}

	spToken, settings, err := token.GetServicePrincipalTokenFromEnvironment()
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire sp token %w", err)
	}
	refreshToken, err := registry.GetRegistryRefreshTokenFromAADExchange(serverURL, spToken, settings.Values[auth.TenantID])
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire refresh token %w", err)
	}
	return acrTokenUsername, refreshToken, nil
}
