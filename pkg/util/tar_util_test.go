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

package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleContainerTools/kaniko/pkg/filesystem"
	"github.com/GoogleContainerTools/kaniko/testutil"
)

var (
	regularFiles     = []string{"file", "file.tar", "file.tar.gz"}
	uncompressedTars = []string{"uncompressed", "uncompressed.tar"}
	compressedTars   = []string{"compressed", "compressed.tar.gz"}
)

func Test_IsLocalTarArchive(t *testing.T) {
	testDir := t.TempDir()
	if err := setUpFilesAndTars(testDir); err != nil {
		t.Fatal(err)
	}
	// Test we get the correct result for regular files
	for _, regularFile := range regularFiles {
		isTarArchive := IsFileLocalTarArchive(filepath.Join(testDir, regularFile))
		testutil.CheckErrorAndDeepEqual(t, false, nil, false, isTarArchive)
	}
	// Test we get the correct result for uncompressed tars
	for _, uncompressedTar := range uncompressedTars {
		isTarArchive := IsFileLocalTarArchive(filepath.Join(testDir, uncompressedTar))
		testutil.CheckErrorAndDeepEqual(t, false, nil, true, isTarArchive)
	}
	// Test we get the correct result for compressed tars
	for _, compressedTar := range compressedTars {
		isTarArchive := IsFileLocalTarArchive(filepath.Join(testDir, compressedTar))
		testutil.CheckErrorAndDeepEqual(t, false, nil, true, isTarArchive)
	}
}

func Test_AddFileToTar(t *testing.T) {
	testDir := t.TempDir()

	path := filepath.Join(testDir, regularFiles[0])
	if err := filesystem.WriteFile(path, []byte("hello"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	// use a pre-determined time with non-zero microseconds to avoid flakiness
	mtime := time.UnixMicro(1635533172891395)
	if err := filesystem.FS.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	tarw := NewTar(buf)
	if err := tarw.AddFileToTar(path); err != nil {
		t.Fatal(err)
	}
	tarw.Close()

	// Check that the mtime is correct (#1808)
	tarReader := tar.NewReader(buf)
	hdr, err := tarReader.Next()
	if err != nil {
		t.Fatal(err)
	}
	testutil.CheckDeepEqual(t, mtime, hdr.ModTime)
}

func setUpFilesAndTars(testDir string) error {
	regularFilesAndContents := map[string]string{
		regularFiles[0]: "",
		regularFiles[1]: "something",
		regularFiles[2]: "here",
	}
	if err := testutil.SetupFiles(testDir, regularFilesAndContents); err != nil {
		return err
	}

	for _, uncompressedTar := range uncompressedTars {
		tarFile, err := filesystem.FS.Create(filepath.Join(testDir, uncompressedTar))
		if err != nil {
			return err
		}
		if err := createTar(testDir, tarFile); err != nil {
			return err
		}
	}

	for _, compressedTar := range compressedTars {
		tarFile, err := filesystem.FS.Create(filepath.Join(testDir, compressedTar))
		if err != nil {
			return err
		}
		gzr := gzip.NewWriter(tarFile)
		if err := createTar(testDir, gzr); err != nil {
			return err
		}
	}
	return nil
}

func createTar(testdir string, writer io.Writer) error {
	t := NewTar(writer)
	defer t.Close()
	for _, regFile := range regularFiles {
		filePath := filepath.Join(testdir, regFile)
		if err := t.AddFileToTar(filePath); err != nil {
			return err
		}
	}
	return nil
}

func Test_CreateTarballOfDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	wantErr := false
	createFilesInTempDir(t, tmpDir)
	f := &bytes.Buffer{}
	err := CreateTarballOfDirectory(tmpDir, f)
	testutil.CheckError(t, wantErr, err)

	extracedFilesDir := filepath.Join(tmpDir, "extracted")
	err = filesystem.FS.Mkdir(extracedFilesDir, 0o755)
	if err != nil {
		t.Error(err)
		return
	}
	files, err := UnTar(f, extracedFilesDir)
	testutil.CheckError(t, wantErr, err)
	for _, filePath := range files {
		fileInfo, err := filesystem.FS.Lstat(filePath)
		testutil.CheckError(t, wantErr, err)
		if fileInfo.IsDir() {
			// skip directory
			continue
		}
		if modTime := fileInfo.ModTime(); modTime.Equal(time.Unix(0, 0)) {
			t.Errorf("unexpected modtime %q of %q", modTime, fileInfo.Name())
		}
		file, err := filesystem.FS.Open(filePath)
		testutil.CheckError(t, wantErr, err)
		body, err := io.ReadAll(file)
		testutil.CheckError(t, wantErr, err)
		index := filepath.Base(filePath)
		testutil.CheckDeepEqual(t, string(body), fmt.Sprintf("hello from %s\n", index))
	}
}

func createFilesInTempDir(t *testing.T, tmpDir string) {
	for i := 0; i < 2; i++ {
		fName := filepath.Join(tmpDir, fmt.Sprint(i))
		content := fmt.Sprintf("hello from %d\n", i)
		if err := filesystem.WriteFile(fName, []byte(content), 0o666); err != nil {
			t.Error(err)
			return
		}
	}
}

func Test_NewReproducibleTar(t *testing.T) {
	tmpDir := t.TempDir()
	createFilesInTempDir(t, tmpDir)
	f := &bytes.Buffer{}

	// Create tarball ignoring timestamps
	tw := NewReproducibleTar(f)
	if err := filesystem.WalkDir(tmpDir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("path %v is not absolute", path)
		}
		return tw.AddFileToTar(path)
	}); err != nil {
		t.Fatalf("failed to create tar of %q: %s", tmpDir, err.Error())
	}

	extracedFilesDir := filepath.Join(tmpDir, "extracted")
	if err := filesystem.FS.Mkdir(extracedFilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := UnTar(f, extracedFilesDir)
	if err != nil {
		t.Fatalf("untar: %s", err.Error())
	}
	for _, filePath := range files {
		fileInfo, err := filesystem.FS.Lstat(filePath)
		if err != nil {
			t.Fatalf("stat %q: %s", filePath, err.Error())
		}
		if fileInfo.IsDir() {
			// skip directory
			continue
		}
		// In a 'reproducible' tar, all timestamps should be set to zero
		if modTime := fileInfo.ModTime(); !modTime.Equal(time.Unix(0, 0)) {
			t.Errorf("unexpected modtime %q of %q", modTime, filePath)
		}
		file, err := filesystem.FS.Open(filePath)
		if err != nil {
			t.Fatalf("open file %q: %s", filePath, err.Error())
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read file %q: %s", filePath, err.Error())
		}
		index := filepath.Base(filePath)
		testutil.CheckDeepEqual(t, string(body), fmt.Sprintf("hello from %s\n", index))
	}
}
