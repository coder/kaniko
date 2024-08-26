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

package imagefs

import (
	"archive/tar"
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v5"

	"github.com/GoogleContainerTools/kaniko/pkg/util"
)

type imageFS struct {
	vfs.FS
	image map[string]v1.Image
	dirs  map[string]*cachedDir
	files map[string]*cachedFileInfo
}

func New(parent vfs.FS, root string, image v1.Image, filesToCache []string) (vfs.FS, error) {
	var ifs *imageFS

	// Multiple layers of imageFS might get confusing, enable delayering.
	if fs, ok := parent.(*imageFS); ok {
		if _, ok := fs.image[root]; ok {
			return nil, fmt.Errorf("imagefs: root already exists: %s", root)
		}
		fs.image[root] = image
		ifs = fs
	} else {
		ifs = &imageFS{
			FS:    vfs.NewReadOnlyFS(parent),
			image: map[string]v1.Image{root: image},
			dirs:  make(map[string]*cachedDir),
			files: make(map[string]*cachedFileInfo),
		}
	}

	// Walk the image and cache file info and hash of the requested files.
	_, err := util.GetFSFromImage(root, image, func(dest string, hdr *tar.Header, cleanedName string, tr io.Reader) error {
		for _, f := range filesToCache {
			dest := filepath.Join(root, cleanedName)

			// Check if the file matches the requested file.
			if ok, err := filepath.Match(f, "/"+cleanedName); ok && err == nil {
				logrus.Debugf("imagefs: Found cacheable file %q (%s) (%d:%d)", f, dest, hdr.Uid, hdr.Gid)

				f := newCachedFileInfo(hdr)

				// Hash the file, implementation must match util.CacheHasher.
				h := md5.New()
				h.Write([]byte(f.Mode().String()))
				h.Write([]byte(strconv.FormatUint(uint64(hdr.Uid), 36)))
				h.Write([]byte(","))
				h.Write([]byte(strconv.FormatUint(uint64(hdr.Gid), 36)))
				if f.Mode().IsRegular() {
					if _, err := io.Copy(h, tr); err != nil {
						return err
					}
				} else if f.Mode()&os.ModeSymlink == os.ModeSymlink {
					h.Write([]byte(hdr.Linkname))
				}
				f.md5sum = h.Sum(nil)

				ifs.files[dest] = f

				return nil
			}

			// Parent directories are needed for lookup.
			if cleanedName == "/" || strings.HasPrefix(f, "/"+cleanedName+"/") {
				logrus.Debugf("imagefs: Found cacheable file parent %q (%s)", f, dest)

				ifs.files[dest] = newCachedFileInfo(hdr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk image")
	}

	for dir, d := range ifs.files {
		if !d.IsDir() {
			continue
		}
		ifs.dirs[dir] = &cachedDir{FileInfo: d}
		for name, fi := range ifs.files {
			if filepath.Dir(name) == dir {
				ifs.dirs[dir].entry = append(ifs.dirs[dir].entry, fi)
			}
		}
	}

	return ifs, nil
}

func (ifs *imageFS) Open(name string) (fs.File, error) {
	logrus.Debugf("imagefs: Open file %s", name)
	if f, err := ifs.FS.Open(name); err == nil {
		return f, nil
	}
	if ifs.files[name] != nil {
		logrus.Debugf("imagefs: Open cached file %s", name)
		return ifs.files[name], nil
	}
	return nil, fs.ErrNotExist
}

func (ifs *imageFS) Lstat(name string) (fs.FileInfo, error) {
	logrus.Debugf("imagefs: Lstat file %s", name)
	if fi, err := ifs.FS.Lstat(name); err == nil {
		return fi, nil
	}
	if ifs.files[name] != nil {
		logrus.Debugf("imagefs: Lstat cached file %s", name)
		return ifs.files[name], nil
	}
	return nil, fs.ErrNotExist
}

func (ifs *imageFS) Stat(name string) (fs.FileInfo, error) {
	logrus.Debugf("imagefs: Stat file %s", name)
	if fi, err := ifs.FS.Stat(name); err == nil {
		return fi, nil
	}
	if ifs.files[name] != nil {
		logrus.Debugf("imagefs: Stat cached file %s", name)
		return ifs.files[name], nil
	}
	return nil, fs.ErrNotExist
}

func (ifs *imageFS) ReadDir(name string) ([]fs.DirEntry, error) {
	logrus.Debugf("imagefs: Reading directory %s", name)
	if de, err := ifs.FS.ReadDir(name); err == nil {
		return de, nil
	}
	for dir, d := range ifs.dirs {
		if ok, err := filepath.Match(name, dir); ok && err == nil {
			logrus.Debugf("imagefs: Reading cached directory %s", name)
			return d.entry, nil
		}
	}
	return nil, fs.ErrNotExist
}

type cachedDir struct {
	fs.FileInfo
	entry []fs.DirEntry
}

type cachedFileInfo struct {
	fs.FileInfo
	hdr    *tar.Header
	sys    interface{}
	md5sum []byte
}

// Ensure that cachedFileInfo implements the CacheHasherFileInfoSum interface.
var _ util.CacheHasherFileInfoSum = &cachedFileInfo{}

func newCachedFileInfo(hdr *tar.Header) *cachedFileInfo {
	fi := hdr.FileInfo()
	return &cachedFileInfo{
		FileInfo: fi,
		hdr:      hdr,
		sys: &syscall.Stat_t{
			Uid: uint32(hdr.Uid),
			Gid: uint32(hdr.Gid),
		},
	}
}

func (cf *cachedFileInfo) Sys() interface{} {
	logrus.Debugf("imagefs: Sys cached file %s", cf.Name())
	return cf.sys
}

func (cf *cachedFileInfo) Stat() (fs.FileInfo, error) {
	logrus.Debugf("imagefs: Stat cached file %s", cf.Name())
	return cf, nil
}

func (cf *cachedFileInfo) Read(p []byte) (n int, err error) {
	panic("imagefs: Read cached file is not allowed")
}

func (cf *cachedFileInfo) MD5Sum() ([]byte, error) {
	logrus.Debugf("imagefs: MD5Sum cached file %s", cf.Name())
	return cf.md5sum, nil
}

func (cf *cachedFileInfo) Type() fs.FileMode {
	logrus.Debugf("imagefs: Type cached file %s", cf.Name())
	return cf.Mode()
}

func (cf *cachedFileInfo) Info() (fs.FileInfo, error) {
	logrus.Debugf("imagefs: Info cached file %s", cf.Name())
	return cf, nil
}

func (cf *cachedFileInfo) Close() error {
	logrus.Debugf("imagefs: Close cached file %s", cf.Name())
	return nil
}
