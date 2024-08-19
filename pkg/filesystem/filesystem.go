package filesystem

import (
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"

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
func Walk(root string, walkFn filepath.WalkFunc) error { return afero.Walk(FS, root, walkFn) }

// WalkDir implements io/fs.WalkDir.
func WalkDir(root string, walkFn iofs.WalkDirFunc) error { return iofs.WalkDir(IOFS(), root, walkFn) }

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(FS, filename, data, perm)
}
