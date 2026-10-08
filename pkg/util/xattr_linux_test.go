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

package util

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestXattrRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	const attr = "user.kaniko.test"

	got, err := lgetxattr(path, attr)
	if errors.Is(err, syscall.EOPNOTSUPP) {
		t.Skip("filesystem does not support user xattrs")
	}
	if err != nil || got != nil {
		t.Fatalf("lgetxattr on unset attribute = (%q, %v), want (nil, nil)", got, err)
	}

	for _, want := range [][]byte{
		[]byte("small"),
		// Larger than the initial 128-byte buffer, exercising the ERANGE retry.
		bytes.Repeat([]byte("x"), 300),
	} {
		if err := lsetxattr(path, attr, want, 0); err != nil {
			if errors.Is(err, syscall.EOPNOTSUPP) {
				t.Skip("filesystem does not support user xattrs")
			}
			t.Fatalf("lsetxattr: %v", err)
		}
		got, err := lgetxattr(path, attr)
		if err != nil {
			t.Fatalf("lgetxattr: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("lgetxattr = %d bytes, want %d bytes", len(got), len(want))
		}
	}
}

func TestXattrErrorsWrapErrno(t *testing.T) {
	_, err := lgetxattr(filepath.Join(t.TempDir(), "missing"), "user.kaniko.test")
	if !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("lgetxattr on missing file = %v, want an error wrapping ENOENT", err)
	}
	err = lsetxattr(filepath.Join(t.TempDir(), "missing"), "user.kaniko.test", []byte("v"), 0)
	if !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("lsetxattr on missing file = %v, want an error wrapping ENOENT", err)
	}
}
