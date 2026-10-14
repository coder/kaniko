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

// Ported from https://github.com/chrismellard/docker-credential-acr-env/blob/82a0ddb27589/pkg/registry/const.go
// (Copyright 2020 Chris Mellard, Apache License 2.0) so that kaniko does not
// depend on github.com/chrismellard/docker-credential-acr-env (GO-2026-6225).

package acr

import (
	"time"
)

const (
	secureScheme   = "https://"
	defaultTimeOut = time.Duration(30) * time.Second
)
