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

// The first group of cases is from
// https://github.com/chrismellard/docker-credential-acr-env/blob/82a0ddb27589/pkg/credhelper/helper_test.go
// (Copyright 2022 Chris Mellard, Apache License 2.0).

package acr

import "testing"

func TestIsACRRegistry(t *testing.T) {
	for _, c := range []struct {
		url  string
		want bool
	}{
		// Upstream cases.
		{"myregistry.azurecr.io", true},
		{"myregistry.azurecr.cn", true},
		{"myregistry.azurecr.de", true},
		{"myregistry.azurecr.us", true},
		{"mcr.microsoft.com", true},
		{"myregistry.azurecr.me", false}, // not a known tld
		{"notacr.xcr.example", false},
		{"127.0.0.1:12345", false},
		{"localhost:12345", false},
		{"notaurl-)(*$@)(*@)(*", false},
		// Forms that keep working.
		{"myregistry.azurecr.io:443", true},
		{"myregistry.azurecr.io.", true},
		{"mcr.microsoft.com.", true},
		{"MyRegistry.azurecr.io", true},
		{"my-registry.azurecr.io", true},
		{"myregistry.westus.data.azurecr.io", true},
		{"myregistry.privatelink.azurecr.io", true},
		{"user:pw@myregistry.azurecr.io", true},
		// GO-2026-6225: hosts that contain an ACR domain without ending in one.
		{"evil.azurecr.io.attacker.com", false},
		{"myregistry.azurecr.io.attacker.com:443", false},
		{"azurecr.io.attacker.com", false},
		{"mcr.microsoft.com.attacker.com", false},
		{"myregistry.azurecr.iox", false},
		{"attacker.com/myregistry.azurecr.io", false},
		{"myregistry.azurecr.io@attacker.com", false},
		{"attacker.com#.azurecr.io", false},
		{"attacker.com?.azurecr.io", false},
		// Strings that are not DNS names.
		{"azurecr.io", false},
		{".azurecr.io", false},
		{"a..azurecr.io", false},
		{"-a.azurecr.io", false},
		{"a-.azurecr.io", false},
		{"attacker.com%25.azurecr.io", false},
		{"[::ffff:127.0.0.1%25x.azurecr.io]", false},
		{"[::ffff:127.0.0.1%25x.azurecr.io]:443", false},
		{"MYREGISTRY.AZURECR.IO", false}, // upstream is case sensitive here too
	} {
		t.Run(c.url, func(t *testing.T) {
			if got := isACRRegistry(c.url); got != c.want {
				t.Fatalf("isACRRegistry(%q) = %t, want %t", c.url, got, c.want)
			}
		})
	}
}

func TestACRCredHelperRejectsNonACRHost(t *testing.T) {
	// Get must reject non-ACR hosts before reading Azure credentials from the
	// environment. Checking the error text distinguishes the host check from a
	// later failure of the token exchange with these fake credentials.
	t.Setenv("AZURE_TENANT_ID", "tenant")
	t.Setenv("AZURE_CLIENT_ID", "client")
	t.Setenv("AZURE_CLIENT_SECRET", "secret")
	user, pass, err := NewACRCredentialsHelper().Get("evil.azurecr.io.attacker.com")
	if err == nil || err.Error() != "serverURL does not refer to Azure Container Registry" || user != "" || pass != "" {
		t.Fatalf("Get returned (%q, %q, %v), want the non-ACR host error and no credentials", user, pass, err)
	}
}
