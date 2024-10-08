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
	"sync"
	"syscall"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v5"

	"github.com/GoogleContainerTools/kaniko/pkg/util"
)

type imageFS struct {
	vfs.FS

	mu    sync.RWMutex // Protects following.
	image map[string]v1.Image
	dirs  map[string]*cachedDir
	files map[string]imageFSFile
}

type imageFSFile interface {
	fs.File
	fs.FileInfo
	fs.DirEntry
}

func New(parent vfs.FS, root string, image v1.Image, filesToCache []string) (vfs.FS, error) {
	if image == nil {
		return nil, errors.New("imagefs: image cannot be nil")
	}

	layers, err := image.Layers()
	if err != nil {
		return nil, errors.Wrap(err, "imagefs: get layers failed")
	}

	var ifs *imageFS
	// Multiple layers of imageFS might get confusing, enable delayering.
	if pfs, ok := parent.(*imageFS); ok {
		pfs.mu.Lock()
		defer pfs.mu.Unlock()

		if _, ok := pfs.image[root]; ok {
			return nil, fmt.Errorf("imagefs: root already exists: %s", root)
		}
		pfs.image[root] = image
		ifs = pfs
	} else {
		ifs = &imageFS{
			FS:    vfs.NewReadOnlyFS(parent),
			image: map[string]v1.Image{root: image},
			dirs:  make(map[string]*cachedDir),
			files: make(map[string]imageFSFile),
		}
	}

	logrus.Debugf("imagefs: Caching files for %s", root)

	// Keep track of directories so we can cache all of their contents.
	var dirsToCache []string
	// Walk the image and cache file info and hash of the requested files.
	_, err = util.GetFSFromLayers(root, layers, util.ExtractFunc(func(dest string, hdr *tar.Header, cleanedName string, tr io.Reader) error {
		// Trim prefix for consistent path.
		cleanedName = strings.TrimPrefix(cleanedName, "/")
		path := filepath.Join(dest, cleanedName)

		cacheFile := func() error {
			logrus.Debugf("imagefs: Found cacheable file /%s (path=%s) (%d:%d)", cleanedName, path, hdr.Uid, hdr.Gid)

			cf := newCachedFileInfo(path, hdr)
			if cf.IsDir() {
				dirsToCache = append(dirsToCache, cleanedName)
			}

			sum, err := hashFile(hdr, tr)
			if err != nil {
				return errors.Wrap(err, "imagefs: hash file failed")
			}
			ifs.files[path] = newCachedFileInfoWithMD5Sum(cf, sum)

			return nil
		}

		// All files inside a cached directory should be cached as well.
		for _, dir := range dirsToCache {
			if strings.HasPrefix(cleanedName, dir+"/") {
				return cacheFile()
			}
		}

		for _, f := range filesToCache {
			f = strings.TrimPrefix(f, "/")
			f = strings.TrimSuffix(f, "/")

			// Check if the file matches the requested file.
			if ok, err := filepath.Match(f, cleanedName); ok && err == nil {
				return cacheFile()
			}

			// Cache parent directories for directory lookups.
			if cleanedName == "" || strings.HasPrefix(f, cleanedName+"/") {
				if _, ok := ifs.files[path]; !ok {
					logrus.Debugf("imagefs: Found cacheable file parent /%s (file=/%s)", cleanedName, f)
					ifs.files[path] = newCachedFileInfo(dest, hdr)
				}
			}
		}

		return nil
	}))
	if err != nil {
		return nil, errors.Wrap(err, "imagefs: walk image failed")
	}

	logrus.Debugf("imagefs: Creating cached directories for %s", root)

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

	logrus.Debugf("imagefs: Cached files for %s", root)

	return ifs, nil
}

func (ifs *imageFS) Open(name string) (fs.File, error) {
	logrus.Debugf("imagefs: Open file %s", name)
	if f, err := ifs.FS.Open(name); err == nil {
		return f, nil
	}

	ifs.mu.RLock()
	defer ifs.mu.RUnlock()
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

	ifs.mu.RLock()
	defer ifs.mu.RUnlock()
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

	ifs.mu.RLock()
	defer ifs.mu.RUnlock()
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

	ifs.mu.RLock()
	defer ifs.mu.RUnlock()
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
	path string
	hdr  *tar.Header
	sys  *syscall.Stat_t
}

func newCachedFileInfo(path string, hdr *tar.Header) *cachedFileInfo {
	return &cachedFileInfo{
		FileInfo: hdr.FileInfo(),
		path:     path,
		hdr:      hdr,
		sys:      tarHeaderToStat_t(hdr),
	}
}

func (cf *cachedFileInfo) Sys() interface{} {
	logrus.Debugf("imagefs: Sys cached file: %s", cf.path)
	return cf.sys
}

func (cf *cachedFileInfo) Stat() (fs.FileInfo, error) {
	logrus.Debugf("imagefs: Stat cached file: %s", cf.path)
	return cf, nil
}

func (cf *cachedFileInfo) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("imagefs: Read cached file is not allowed: %s", cf.path)
}

func (cf *cachedFileInfo) Type() fs.FileMode {
	logrus.Debugf("imagefs: Type cached file: %s", cf.path)
	return cf.Mode()
}

func (cf *cachedFileInfo) Info() (fs.FileInfo, error) {
	logrus.Debugf("imagefs: Info cached file: %s", cf.path)
	return cf, nil
}

func (cf *cachedFileInfo) Close() error {
	logrus.Debugf("imagefs: Close cached file: %s", cf.path)
	return nil
}

type cachedFileInfoWithMD5Sum struct {
	*cachedFileInfo
	md5sum []byte
}

func newCachedFileInfoWithMD5Sum(fi *cachedFileInfo, md5sum []byte) *cachedFileInfoWithMD5Sum {
	return &cachedFileInfoWithMD5Sum{
		cachedFileInfo: fi,
		md5sum:         md5sum,
	}
}

// Ensure that cachedFileInfo implements the CacheHasherFileInfoSum interface.
var _ util.CacheHasherFileInfoSum = &cachedFileInfoWithMD5Sum{}

func (cf *cachedFileInfoWithMD5Sum) MD5Sum() ([]byte, error) {
	logrus.Debugf("imagefs: MD5Sum cached file: %s", cf.path)
	return cf.md5sum, nil
}

// tarHeaderToStat_t converts a tar.Header to a syscall.Stat_t.
func tarHeaderToStat_t(hdr *tar.Header) *syscall.Stat_t {
	fi := hdr.FileInfo()
	return &syscall.Stat_t{
		Mode: uint32(fi.Mode()),
		Uid:  uint32(hdr.Uid),
		Gid:  uint32(hdr.Gid),
		Size: fi.Size(),
		Atim: timespec(hdr.AccessTime),
		Ctim: timespec(hdr.ChangeTime),
		Mtim: timespec(fi.ModTime()),
	}
}

// hashFile hashes the gievn file, implementation must match util.CacheHasher.
func hashFile(hdr *tar.Header, r io.Reader) ([]byte, error) {
	fi := hdr.FileInfo()

	h := md5.New()
	h.Write([]byte(fi.Mode().String()))
	h.Write([]byte(strconv.FormatUint(uint64(hdr.Uid), 36)))
	h.Write([]byte(","))
	h.Write([]byte(strconv.FormatUint(uint64(hdr.Gid), 36)))
	if fi.Mode().IsRegular() {
		if _, err := io.Copy(h, r); err != nil {
			return nil, errors.Wrap(err, "imagefs: copy file content failed")
		}
	} else if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		h.Write([]byte(hdr.Linkname))
	}
	return h.Sum(nil), nil
}
