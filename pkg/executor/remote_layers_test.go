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

package executor

import (
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleContainerTools/kaniko/pkg/config"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Test_saveStageAsTarball_RemoteLayersExceedPullLimit ensures saving a remote
// image with more layers than the go-containerregistry pull limit completes.
// Each open remote blob reader holds a limiter slot, so any reader that is not
// closed blocks later layer fetches on the same image forever.
func Test_saveStageAsTarball_RemoteLayersExceedPullLimit(t *testing.T) {
	s := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(u.Host + "/kaniko/stage:latest")
	if err != nil {
		t.Fatal(err)
	}

	const numLayers = 6
	img, err := random.Image(64, numLayers)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}
	pulled, err := remote.Image(ref, remote.WithJobs(2))
	if err != nil {
		t.Fatal(err)
	}

	original := config.KanikoIntermediateStagesDir
	config.KanikoIntermediateStagesDir = t.TempDir()
	defer func() { config.KanikoIntermediateStagesDir = original }()

	done := make(chan error, 1)
	go func() {
		done <- saveStageAsTarball("0", pulled)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out saving stage; remote layer readers are likely not closed")
	}

	saved, err := tarball.ImageFromPath(filepath.Join(config.KanikoIntermediateStagesDir, "0"), nil)
	if err != nil {
		t.Fatal(err)
	}
	layers, err := saved.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != numLayers {
		t.Fatalf("expected %d layers in saved stage, got %d", numLayers, len(layers))
	}
}
