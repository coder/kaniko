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

package creds

import (
	"strings"
	"testing"

	"github.com/osscontainertools/docker-credential-acr/pkg/credhelper"
)

// The previous ACR helper matched any host containing ".azurecr.io" and sent
// the Azure AD token to it. These hosts must be rejected before any token
// lookup happens.
func TestACRHelperRejectsLookalikeHosts(t *testing.T) {
	hosts := []string{
		"evil.azurecr.io.attacker.com",
		"evil.azurecr.cn.attacker.com",
		"azurecr.io.attacker.com",
		"example.com",
	}
	helper := credhelper.NewACRCredentialsHelper()
	for _, host := range hosts {
		_, _, err := helper.Get(host)
		if err == nil || !strings.Contains(err.Error(), "does not refer to Azure Container Registry") {
			t.Errorf("Get(%q): expected the host to be rejected, got err=%v", host, err)
		}
	}
}
