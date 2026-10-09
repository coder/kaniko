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

// Command docker-credential-acr-env is a Docker credential helper for Azure
// Container Registry. It replaces github.com/chrismellard/docker-credential-acr-env
// in the kaniko images; see pkg/creds/acr.
package main

import (
	"github.com/GoogleContainerTools/kaniko/pkg/creds/acr"
	"github.com/docker/docker-credential-helpers/credentials"
)

func main() {
	credentials.Serve(acr.NewACRCredentialsHelper())
}
