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

package commands

import (
	"archive/tar"
	"bytes"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/GoogleContainerTools/kaniko/pkg/dockerfile"
	"github.com/GoogleContainerTools/kaniko/pkg/filesystem"
	"github.com/GoogleContainerTools/kaniko/testutil"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
)

func Test_addDefaultHOME(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		mockUser    *user.User
		lookupError error
		initial     []string
		expected    []string
	}{
		{
			name: "HOME already set",
			user: "",
			initial: []string{
				"HOME=/something",
				"PATH=/something/else",
			},
			expected: []string{
				"HOME=/something",
				"PATH=/something/else",
			},
		},
		{
			name: "HOME not set and user not set",
			user: "",
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/root",
			},
		},
		{
			name: "HOME not set and user and homedir for the user set",
			user: "www-add",
			mockUser: &user.User{
				Username: "www-add",
				HomeDir:  "/home/some-other",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/home/some-other",
			},
		},
		{
			name: "USER is set using the UID",
			user: "1000",
			mockUser: &user.User{
				Username: "1000",
				HomeDir:  "/",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/",
			},
		},
		{
			name: "HOME not set and user is set to root",
			user: "root",
			mockUser: &user.User{
				Username: "root",
			},
			initial: []string{
				"PATH=/something/else",
			},
			expected: []string{
				"PATH=/something/else",
				"HOME=/root",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := userLookup
			userLookup = func(username string) (*user.User, error) { return test.mockUser, test.lookupError }
			defer func() {
				userLookup = original
			}()
			actual, err := addDefaultHOME(test.user, test.initial)
			testutil.CheckErrorAndDeepEqual(t, false, err, test.expected, actual)
		})
	}
}

func prepareTarFixture(t *testing.T, fileNames []string) ([]byte, error) {
	dir := t.TempDir()

	content := `
Meow meow meow meow
meow meow meow meow
`
	for _, name := range fileNames {
		if err := filesystem.WriteFile(filepath.Join(dir, name), []byte(content), 0o777); err != nil {
			return nil, err
		}
	}
	writer := bytes.NewBuffer([]byte{})
	tw := tar.NewWriter(writer)
	defer tw.Close()
	filesystem.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatal(err)
		}
		body, err := filesystem.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := tw.Write(body); err != nil {
			log.Fatal(err)
		}

		return nil
	})

	return writer.Bytes(), nil
}

func Test_CachingRunCommand_ExecuteCommand(t *testing.T) {
	tarContent, err := prepareTarFixture(t, []string{"foo.txt"})
	if err != nil {
		t.Errorf("couldn't prepare tar fixture %v", err)
	}

	config := &v1.Config{}
	buildArgs := &dockerfile.BuildArgs{}

	type testCase struct {
		desctiption    string
		expectLayer    bool
		expectErr      bool
		count          *int
		expectedCount  int
		command        *CachingRunCommand
		extractedFiles []string
		contextFiles   []string
	}
	testCases := []testCase{
		func() testCase {
			c := &CachingRunCommand{
				img: fakeImage{
					ImageLayers: []v1.Layer{
						fakeLayer{TarContent: tarContent},
					},
				},
			}
			count := 0
			tc := testCase{
				desctiption:    "with valid image and valid layer",
				count:          &count,
				expectedCount:  1,
				expectLayer:    true,
				extractedFiles: []string{"/foo.txt"},
				contextFiles:   []string{"foo.txt"},
			}
			c.extractFn = func(_ string, _ *tar.Header, _ string, _ io.Reader) error {
				*tc.count++
				return nil
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CachingRunCommand{}
			tc := testCase{
				desctiption: "with no image",
				expectErr:   true,
			}
			c.extractFn = func(_ string, _ *tar.Header, _ string, _ io.Reader) error {
				return nil
			}
			tc.command = c
			return tc
		}(),
		func() testCase {
			c := &CachingRunCommand{
				img: fakeImage{},
			}

			c.extractFn = func(_ string, _ *tar.Header, _ string, _ io.Reader) error {
				return nil
			}

			return testCase{
				desctiption: "with image containing no layers",
				expectErr:   true,
				command:     c,
			}
		}(),
		func() testCase {
			c := &CachingRunCommand{
				img: fakeImage{
					ImageLayers: []v1.Layer{
						fakeLayer{},
					},
				},
			}
			c.extractFn = func(_ string, _ *tar.Header, _ string, _ io.Reader) error {
				return nil
			}
			tc := testCase{
				desctiption: "with image one layer which has no tar content",
				expectErr:   false, // this one probably should fail but doesn't because of how ExecuteCommand and util.GetFSFromLayers are implemented - cvgw- 2019-11-25
				expectLayer: true,
			}
			tc.command = c
			return tc
		}(),
	}

	for _, tc := range testCases {
		t.Run(tc.desctiption, func(t *testing.T) {
			c := tc.command
			err := c.ExecuteCommand(config, buildArgs)
			if !tc.expectErr && err != nil {
				t.Errorf("Expected err to be nil but was %v", err)
			} else if tc.expectErr && err == nil {
				t.Error("Expected err but was nil")
			}

			if tc.count != nil {
				if *tc.count != tc.expectedCount {
					t.Errorf("Expected extractFn to be called %v times but was called %v times", 1, *tc.count)
				}
				for _, file := range tc.extractedFiles {
					match := false
					cmdFiles := c.extractedFiles
					for _, f := range cmdFiles {
						if file == f {
							match = true
							break
						}
					}
					if !match {
						t.Errorf("Expected extracted files to include %v but did not %v", file, cmdFiles)
					}
				}

				// CachingRunCommand does not override BaseCommand
				// FilesUseFromContext so this will always return an empty slice and no error
				// This seems like it might be a bug as it results in RunCommands and CachingRunCommands generating different cache keys - cvgw - 2019-11-27
				cmdFiles, err := c.FilesUsedFromContext(
					config, buildArgs,
				)
				if err != nil {
					t.Errorf("failed to get files used from context from command")
				}

				if len(cmdFiles) != 0 {
					t.Errorf("expected files used from context to be empty but was not")
				}
			}

			if c.layer == nil && tc.expectLayer {
				t.Error("expected the command to have a layer set but instead was nil")
			} else if c.layer != nil && !tc.expectLayer {
				t.Error("expected the command to have no layer set but instead found a layer")
			}
		})
	}
}

func TestFileCreatorCleaner(t *testing.T) {
	const (
		fileContent = "content"
		fileMode    = 0600
		dirMode     = 0700
	)

	t.Run("Creator", func(t *testing.T) {
		dir := t.TempDir()
		fcc := fileCreatorCleaner{}

		// Given a fileCreatorCleaner
		// When fileCreatorCleaner.MkdirAndWriteFile is called
		file := filepath.Join(dir, "file")
		err := fcc.MkdirAndWriteFile(file, []byte(fileContent), dirMode, fileMode)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Then the file should exist
		if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected file to exist, but it did not")
		}

		// And the file should have the correct content
		content, err := filesystem.ReadFile(file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(content) != fileContent {
			t.Fatalf("expected content to be '%s', but got '%s'", fileContent, content)
		}

		// And the file should have the correct permissions
		info, err := os.Stat(file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Mode() != fileMode {
			t.Fatalf("expected file to have mode %v, but got %v", fileMode, info.Mode())
		}
	})

	t.Run("Cleaner", func(t *testing.T) {
		dir := t.TempDir()
		fcc := fileCreatorCleaner{}

		// Given a fileCreatorCleaner has created a file three directories deep
		file1 := filepath.Join(dir, "a", "b", "c", "file1")
		err := fcc.MkdirAndWriteFile(file1, []byte(fileContent), dirMode, fileMode)
		if err != nil {
			t.Fatalf("creating file using fileCreatorCleaner: %v", err)
		}

		// And the same fileCreatorCleaner has created another file in the same directory
		// This tests that we don't try to remove the same directory twice, which would cause an error
		// on the second removal
		file2 := filepath.Join(dir, "a", "b", "c", "file2")
		err = fcc.MkdirAndWriteFile(file2, []byte(fileContent), dirMode, fileMode)
		if err != nil {
			t.Fatalf("creating file using2 fileCreatorCleaner: %v", err)
		}

		// And the same fileCreatorCleaner has created a file two directories deep into
		// the same path
		file3 := filepath.Join(dir, "a", "b", "file")
		err = fcc.MkdirAndWriteFile(file3, []byte(fileContent), dirMode, fileMode)
		if err != nil {
			t.Fatalf("creating file using fileCreatorCleaner: %v", err)
		}

		// When fileCreatorCleaner.Clean is called
		err = fcc.Clean()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Then all the directories should be removed
		if _, err := os.Stat(filepath.Join(dir, "a")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected directory to be removed, but it was not")
		}

		// And the files should be removed
		for _, file := range []string{file1, file2, file3} {
			if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected file to be removed, but it was not")
			}
		}
	})

	t.Run("Directories with third party contents are not cleaned up", func(t *testing.T) {
		dir := t.TempDir()
		fcc := fileCreatorCleaner{}

		// Given a fileCreatorCleaner has created a file three directories deep
		file := filepath.Join(dir, "a", "b", "c", "file")
		err := fcc.MkdirAndWriteFile(file, []byte(fileContent), dirMode, fileMode)
		if err != nil {
			t.Fatalf("creating file using fileCreatorCleaner: %v", err)
		}

		// And a third party file is created two levels deep
		thirdPartyFile := filepath.Join(dir, "a", "b", "third-party-file")
		err = filesystem.WriteFile(thirdPartyFile, []byte(fileContent), fileMode)
		if err != nil {
			t.Fatalf("creating third party file %v", err)
		}

		// When fileCreatorCleaner.Clean is called
		err = fcc.Clean()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Then the file that was created by the fileCreatorCleaner should be removed
		if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected file to be removed, but it was not")
		}

		// And the now empty third directory should be removed
		if _, err := os.Stat(filepath.Join(dir, "a", "b", "c")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected directory to be removed, but it was not")
		}

		// And the third party file should not be removed, nor should the directories that contain it
		if _, err := os.Stat(thirdPartyFile); errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected third party file to not be removed, but it was")
		}
		if _, err := os.Stat(filepath.Join(dir, "a", "b")); errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected directory to not be removed, but it was")
		}
	})
}

func TestSetWorkDirIfExists(t *testing.T) {
	testDir := t.TempDir()
	testutil.CheckDeepEqual(t, testDir, setWorkDirIfExists(testDir))
	testutil.CheckDeepEqual(t, "", setWorkDirIfExists("doesnot-exists"))
}
