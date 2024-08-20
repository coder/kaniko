package filesystem

import (
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/twpayne/go-vfs/v5"
)

// FS is the default filesystem used by the package.
var FS vfs.FS = vfs.OSFS

func SetFS(fs vfs.FS) {
	FS = fs
}

func IOFS() iofs.FS {
	return FS
}

// TODO(mafredri): Replace with non-os specific functions.
func CreateTemp(dir, pattern string) (*os.File, error) { return os.CreateTemp(dir, pattern) }
func MkdirTemp(dir, pattern string) (string, error)    { return os.MkdirTemp(dir, pattern) }

func ReadFile(filename string) ([]byte, error)           { return FS.ReadFile(filename) }
func ReadDir(name string) ([]iofs.DirEntry, error)       { return FS.ReadDir(name) }
func MkdirAll(path string, perm iofs.FileMode) error     { return vfs.MkdirAll(FS, path, perm) }
func Walk(root string, walkFn filepath.WalkFunc) error   { return vfs.Walk(FS, root, walkFn) }
func WalkDir(root string, walkFn iofs.WalkDirFunc) error { return iofs.WalkDir(FS, root, walkFn) }
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return FS.WriteFile(filename, data, perm)
}
