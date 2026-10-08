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

// Ported unchanged from https://github.com/moby/moby/blob/v27.3.1/builder/dockerfile/buildargs_test.go
// (Copyright 2013-2018 Docker, Inc., Apache License 2.0) so that kaniko does not
// depend on the github.com/docker/docker daemon packages.

package buildargs

import (
	"bytes"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func strPtr(source string) *string {
	return &source
}

func TestGetAllAllowed(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ArgNotUsedInDockerfile":              strPtr("fromopt1"),
		"ArgOverriddenByOptions":              strPtr("fromopt2"),
		"ArgNoDefaultInDockerfileFromOptions": strPtr("fromopt3"),
		"HTTP_PROXY":                          strPtr("theproxy"),
		"all_proxy":                           strPtr("theproxy2"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", strPtr("frommeta1"))
	buildArgs.AddMetaArg("ArgFromMetaOverridden", strPtr("frommeta2"))
	buildArgs.AddMetaArg("ArgFromMetaNotUsed", strPtr("frommeta3"))

	buildArgs.AddArg("ArgOverriddenByOptions", strPtr("fromdockerfile2"))
	buildArgs.AddArg("ArgWithDefaultInDockerfile", strPtr("fromdockerfile1"))
	buildArgs.AddArg("ArgNoDefaultInDockerfile", nil)
	buildArgs.AddArg("ArgNoDefaultInDockerfileFromOptions", nil)
	buildArgs.AddArg("ArgFromMeta", nil)
	buildArgs.AddArg("ArgFromMetaOverridden", strPtr("fromdockerfile3"))

	all := buildArgs.GetAllAllowed()
	expected := map[string]string{
		"HTTP_PROXY":                          "theproxy",
		"all_proxy":                           "theproxy2",
		"ArgOverriddenByOptions":              "fromopt2",
		"ArgWithDefaultInDockerfile":          "fromdockerfile1",
		"ArgNoDefaultInDockerfileFromOptions": "fromopt3",
		"ArgFromMeta":                         "frommeta1",
		"ArgFromMetaOverridden":               "fromdockerfile3",
	}
	assert.Check(t, is.DeepEqual(expected, all))
}

func TestGetAllMeta(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ArgNotUsedInDockerfile":        strPtr("fromopt1"),
		"ArgOverriddenByOptions":        strPtr("fromopt2"),
		"ArgNoDefaultInMetaFromOptions": strPtr("fromopt3"),
		"HTTP_PROXY":                    strPtr("theproxy"),
	})

	buildArgs.AddMetaArg("ArgFromMeta", strPtr("frommeta1"))
	buildArgs.AddMetaArg("ArgOverriddenByOptions", strPtr("frommeta2"))
	buildArgs.AddMetaArg("ArgNoDefaultInMetaFromOptions", nil)

	all := buildArgs.GetAllMeta()
	expected := map[string]string{
		"HTTP_PROXY":                    "theproxy",
		"ArgFromMeta":                   "frommeta1",
		"ArgOverriddenByOptions":        "fromopt2",
		"ArgNoDefaultInMetaFromOptions": "fromopt3",
	}
	assert.Check(t, is.DeepEqual(expected, all))
}

func TestWarnOnUnusedBuildArgs(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ThisArgIsUsed":    strPtr("fromopt1"),
		"ThisArgIsNotUsed": strPtr("fromopt2"),
		"HTTPS_PROXY":      strPtr("referenced builtin"),
		"HTTP_PROXY":       strPtr("unreferenced builtin"),
	})
	buildArgs.AddArg("ThisArgIsUsed", nil)
	buildArgs.AddArg("HTTPS_PROXY", nil)

	buffer := new(bytes.Buffer)
	buildArgs.WarnOnUnusedBuildArgs(buffer)
	out := buffer.String()
	assert.Assert(t, !strings.Contains(out, "ThisArgIsUsed"), out)
	assert.Assert(t, !strings.Contains(out, "HTTPS_PROXY"), out)
	assert.Assert(t, !strings.Contains(out, "HTTP_PROXY"), out)
	assert.Check(t, is.Contains(out, "ThisArgIsNotUsed"))
}

func TestIsUnreferencedBuiltin(t *testing.T) {
	buildArgs := NewBuildArgs(map[string]*string{
		"ThisArgIsUsed":    strPtr("fromopt1"),
		"ThisArgIsNotUsed": strPtr("fromopt2"),
		"HTTPS_PROXY":      strPtr("referenced builtin"),
		"HTTP_PROXY":       strPtr("unreferenced builtin"),
	})
	buildArgs.AddArg("ThisArgIsUsed", nil)
	buildArgs.AddArg("HTTPS_PROXY", nil)

	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("ThisArgIsUsed"))
	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("ThisArgIsNotUsed"))
	assert.Check(t, buildArgs.IsReferencedOrNotBuiltin("HTTPS_PROXY"))
	assert.Check(t, !buildArgs.IsReferencedOrNotBuiltin("HTTP_PROXY"))
}
