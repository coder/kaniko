/*
Copyright 2018 Google LLC

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
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"

	"github.com/GoogleContainerTools/kaniko/pkg/config"
	"github.com/GoogleContainerTools/kaniko/pkg/constants"
	"github.com/GoogleContainerTools/kaniko/testutil"
)

func TestDoCacheProbe(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		testDir, fn := setupCacheProbeTests(t)
		defer fn()
		dockerFile := `FROM scratch
COPY foo/bar.txt copied/
`
		os.WriteFile(filepath.Join(testDir, "workspace", "Dockerfile"), []byte(dockerFile), 0755)
		regCache := setupCacheRegistry(t)
		opts := &config.KanikoOptions{
			DockerfilePath: filepath.Join(testDir, "workspace", "Dockerfile"),
			SrcContext:     filepath.Join(testDir, "workspace"),
			SnapshotMode:   constants.SnapshotModeFull,
			Cache:          true,
			CacheOptions: config.CacheOptions{
				CacheTTL: time.Hour,
			},
			CacheCopyLayers: true,
			CacheRunLayers:  true,
			CacheRepo:       regCache + "/test",
		}
		_, err := DoCacheProbe(opts)
		if err == nil || !strings.Contains(err.Error(), "uncached command") {
			t.Errorf("unexpected error, got %v", err)
		}
	})

	t.Run("Present", func(t *testing.T) {
		testDir, fn := setupCacheProbeTests(t)
		defer fn()
		dockerFile := `FROM scratch
COPY foo/bar.txt copied/
`
		os.WriteFile(filepath.Join(testDir, "workspace", "Dockerfile"), []byte(dockerFile), 0755)
		regCache := setupCacheRegistry(t)
		opts := &config.KanikoOptions{
			DockerfilePath: filepath.Join(testDir, "workspace", "Dockerfile"),
			SrcContext:     filepath.Join(testDir, "workspace"),
			SnapshotMode:   constants.SnapshotModeRedo,
			Cache:          true,
			CacheOptions: config.CacheOptions{
				CacheTTL: time.Hour,
			},
			CacheCopyLayers: true,
			CacheRunLayers:  true,
			CacheRepo:       regCache + "/test",
		}
		// Populate the cache by doing an initial build
		_, err := DoBuild(opts)
		testutil.CheckNoError(t, err)
		opts.Reproducible = true
		_, err = DoCacheProbe(opts)
		testutil.CheckNoError(t, err)
	})

	t.Run("Partial", func(t *testing.T) {
		testDir, fn := setupCacheProbeTests(t)
		defer fn()
		dockerFile := `FROM scratch
COPY foo/bar.txt copied/
`
		os.WriteFile(filepath.Join(testDir, "workspace", "Dockerfile"), []byte(dockerFile), 0755)
		regCache := setupCacheRegistry(t)
		opts := &config.KanikoOptions{
			DockerfilePath: filepath.Join(testDir, "workspace", "Dockerfile"),
			SrcContext:     filepath.Join(testDir, "workspace"),
			SnapshotMode:   constants.SnapshotModeRedo,
			Cache:          true,
			CacheOptions: config.CacheOptions{
				CacheTTL: time.Hour,
			},
			CacheCopyLayers: true,
			CacheRunLayers:  true,
			CacheRepo:       regCache + "/test",
		}
		_, err := DoBuild(opts)
		testutil.CheckNoError(t, err)
		opts.Reproducible = true

		// Modify the Dockerfile to add some extra steps
		dockerFile = `FROM scratch
COPY foo/bar.txt copied/
COPY foo/baz.txt copied/
`
		os.WriteFile(filepath.Join(testDir, "workspace", "Dockerfile"), []byte(dockerFile), 0755)
		_, err = DoCacheProbe(opts)
		if err == nil || !strings.Contains(err.Error(), "uncached command") {
			t.Errorf("unexpected error, got %v", err)
		}
	})

	t.Run("MultiStage", func(t *testing.T) {
		t.Skip("TODO: https://github.com/coder/envbuilder/issues/230")
		testDir, fn := setupMultistageTests(t)
		defer fn()
		dockerFile := `
		FROM scratch as first
		COPY foo/bam.txt copied/
		ENV test test
		
		From scratch as second
		COPY --from=first copied/bam.txt output/bam.txt`
		os.WriteFile(filepath.Join(testDir, "workspace", "Dockerfile"), []byte(dockerFile), 0755)
		regCache := setupCacheRegistry(t)
		opts := &config.KanikoOptions{
			DockerfilePath: filepath.Join(testDir, "workspace", "Dockerfile"),
			SrcContext:     filepath.Join(testDir, "workspace"),
			SnapshotMode:   constants.SnapshotModeRedo,
			Cache:          true,
			CacheOptions: config.CacheOptions{
				CacheTTL: time.Hour,
			},
			CacheCopyLayers: true,
			CacheRunLayers:  true,
			CacheRepo:       regCache + "/test",
		}
		_, err := DoBuild(opts)
		testutil.CheckNoError(t, err)
		os.WriteFile(filepath.Join(testDir, "workspace", "Dockerfile"), []byte(dockerFile), 0755)
		opts.Reproducible = true
		_, err = DoCacheProbe(opts)
		testutil.CheckNoError(t, err)
		// Check Image has one layer bam.txt
		files, err := readDirectory(filepath.Join(testDir, "output"))
		if err != nil {
			t.Fatal(err)
		}
		testutil.CheckDeepEqual(t, 1, len(files))
		testutil.CheckDeepEqual(t, files[0].Name(), "bam.txt")
	})
}

func setupCacheRegistry(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	testReg := registry.New(registry.WithBlobHandler(registry.NewDiskBlobHandler(tempDir)))
	regSrv := httptest.NewServer(testReg)
	t.Cleanup(func() { regSrv.Close() })
	regSrvURL, err := url.Parse(regSrv.URL)
	testutil.CheckNoError(t, err)
	return fmt.Sprintf("localhost:%s", regSrvURL.Port())
}

func setupCacheProbeTests(t *testing.T) (string, func()) {
	testDir := t.TempDir()
	// Create workspace with files, dirs, and symlinks
	// workspace tree:
	// /root
	//    /kaniko
	//    /workspace
	//     - /foo
	//          - bar.txt
	//          - baz.txt
	if err := os.MkdirAll(filepath.Join(testDir, "kaniko/0"), 0755); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(testDir, "workspace")
	// Make foo
	if err := os.MkdirAll(filepath.Join(workspace, "foo"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(workspace, "foo", "bar.txt")
	if err := os.WriteFile(file, []byte("hello"), 0755); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(workspace, "foo", "baz.txt")
	if err := os.WriteFile(file2, []byte("world"), 0755); err != nil {
		t.Fatal(err)
	}

	// set up config
	config.RootDir = testDir
	config.KanikoDir = fmt.Sprintf("%s/%s", testDir, "kaniko")
	// Write path to ignore list
	if err := os.MkdirAll(filepath.Join(testDir, "proc"), 0755); err != nil {
		t.Fatal(err)
	}
	mFile := filepath.Join(testDir, "proc/mountinfo")
	mountInfo := fmt.Sprintf(
		`36 35 98:0 /kaniko %s/kaniko rw,noatime master:1 - ext3 /dev/root rw,errors=continue
36 35 98:0 /proc %s/proc rw,noatime master:1 - ext3 /dev/root rw,errors=continue
`, testDir, testDir)
	if err := os.WriteFile(mFile, []byte(mountInfo), 0644); err != nil {
		t.Fatal(err)
	}
	config.MountInfoPath = mFile
	return testDir, func() {
		config.RootDir = constants.RootDir
		config.MountInfoPath = constants.MountInfoPath
	}
}
