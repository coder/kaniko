package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		// Populate the cache by doing an initial build
		cacheDir := t.TempDir()
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
			CacheRepo:       "oci:/" + cacheDir,
		}
		_, err := DoCacheProbe(opts)
		if err == nil || !strings.Contains(err.Error(), "not supported in fake build") {
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
		cacheDir := t.TempDir()
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
			CacheRepo:       "oci:/" + cacheDir,
		}
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
		cacheDir := t.TempDir()
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
			CacheRepo:       "oci:/" + cacheDir,
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
		if err == nil || !strings.Contains(err.Error(), "not supported in fake build") {
			t.Errorf("unexpected error, got %v", err)
		}
	})
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
