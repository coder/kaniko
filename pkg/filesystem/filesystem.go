package filesystem

import (
	"fmt"
	"io/fs"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/afero"
)

// FS is the default filesystem used by the package.
var FS = NewFS(afero.NewOsFs())

// Filesystem is an interface that extends afero.Fs with additional
// filesystem operations.
type Filesystem interface {
	afero.Fs

	// Lchown(name string, uid, gid int) error
	Link(oldname, newname string) error
	Lstat(name string) (os.FileInfo, error)
	// Mkfifo(path string, mode uint32) (err error)
	// Mknod(path string, mode uint32, dev int) (err error)
	Readlink(name string) (string, error)
	Symlink(oldname, newname string) error
}

type extendedFS struct {
	afero.Fs
}

func (efs *extendedFS) Link(oldname, newname string) error {
	// TODO(mafredri): Implement Link.
	return os.Link(oldname, newname)
}

func (efs *extendedFS) Lstat(name string) (os.FileInfo, error) {
	fs, ok := efs.Fs.(afero.Lstater)
	if !ok {
		return nil, fmt.Errorf("Lstat is not supported by the filesystem")
	}
	fi, ok, err := fs.LstatIfPossible(name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("Lstat is not supported by the filesystem")
	}
	return fi, nil
}

func (efs *extendedFS) Readlink(name string) (string, error) {
	fs, ok := efs.Fs.(afero.LinkReader)
	if !ok {
		return "", fmt.Errorf("Readlink is not supported by the filesystem")
	}
	return fs.ReadlinkIfPossible(name)
}

func (efs *extendedFS) Symlink(oldname, newname string) error {
	fs, ok := efs.Fs.(afero.Linker)
	if !ok {
		return fmt.Errorf("Symlink is not supported by the filesystem")
	}
	return fs.SymlinkIfPossible(oldname, newname)
}

func NewFS(fs afero.Fs) Filesystem {
	return &extendedFS{Fs: fs}
}

func SetFS(fs afero.Fs) {
	FS = NewFS(fs)
}

func IOFS() iofs.FS {
	return afero.NewIOFS(FS)
}

func CreateTemp(dir, pattern string) (afero.File, error) { return afero.TempFile(FS, dir, pattern) }
func MkdirTemp(dir, pattern string) (string, error)      { return afero.TempDir(FS, dir, pattern) }
func ReadFile(filename string) ([]byte, error)           { return afero.ReadFile(FS, filename) }
func ReadDir(name string) ([]iofs.DirEntry, error)       { return iofs.ReadDir(IOFS(), name) }
func TempDir() (string, error)                           { return afero.TempDir(FS, "", "") }

// Walk implements filepath.Walk.
// func Walk(root string, walkFn filepath.WalkFunc) error { return afero.Walk(FS, root, walkFn) }

// WalkDir implements io/fs.WalkDir.
func WalkDir(root string, walkFn iofs.WalkDirFunc) error { return iofs.WalkDir(IOFS(), root, walkFn) }

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(FS, filename, data, perm)
}

// From stdlib, afero.Walk doesn't match in behavior for dead symlinks.

// Walk walks the file tree rooted at root, calling fn for each file or
// directory in the tree, including root.
//
// All errors that arise visiting files and directories are filtered by fn:
// see the [WalkFunc] documentation for details.
//
// The files are walked in lexical order, which makes the output deterministic
// but requires Walk to read an entire directory into memory before proceeding
// to walk that directory.
//
// Walk does not follow symbolic links.
//
// Walk is less efficient than [WalkDir], introduced in Go 1.16,
// which avoids calling os.Lstat on every visited file or directory.
func Walk(root string, fn filepath.WalkFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = walk(root, info, fn)
	}
	if err == filepath.SkipDir || err == filepath.SkipAll {
		return nil
	}
	return err
}

// walk recursively descends path, calling walkFn.
func walk(path string, info fs.FileInfo, walkFn filepath.WalkFunc) error {
	if !info.IsDir() {
		return walkFn(path, info, nil)
	}

	names, err := readDirNames(path)
	err1 := walkFn(path, info, err)
	// If err != nil, walk can't walk into this directory.
	// err1 != nil means walkFn want walk to skip this directory or stop walking.
	// Therefore, if one of err and err1 isn't nil, walk will return.
	if err != nil || err1 != nil {
		// The caller's behavior is controlled by the return value, which is decided
		// by walkFn. walkFn may ignore err and return nil.
		// If walkFn returns SkipDir or SkipAll, it will be handled by the caller.
		// So walk should return whatever walkFn returns.
		return err1
	}

	for _, name := range names {
		filename := filepath.Join(path, name)
		fileInfo, err := FS.Lstat(filename)
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return err
			}
		} else {
			err = walk(filename, fileInfo, walkFn)
			if err != nil {
				if !fileInfo.IsDir() || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}

// readDirNames reads the directory named by dirname and returns
// a sorted list of directory entry names.
func readDirNames(dirname string) ([]string, error) {
	f, err := FS.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	return names, nil
}
