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
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	kConfig "github.com/GoogleContainerTools/kaniko/pkg/config"
	"github.com/GoogleContainerTools/kaniko/pkg/constants"
	"github.com/GoogleContainerTools/kaniko/pkg/dockerfile"
	"github.com/GoogleContainerTools/kaniko/pkg/filesystem"
	"github.com/GoogleContainerTools/kaniko/pkg/util"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type RunOutput struct {
	Stdout io.Writer
	Stderr io.Writer
}

type RunCommand struct {
	BaseCommand
	cmd          *instructions.RunCommand
	output       *RunOutput
	buildSecrets []string
	shdCache     bool
}

const secretsDir = "/run/secrets"

// for testing
var (
	userLookup = util.LookupUser
)

func (r *RunCommand) IsArgsEnvsRequiredInCache() bool {
	return true
}

func (r *RunCommand) ExecuteCommand(config *v1.Config, buildArgs *dockerfile.BuildArgs) error {
	return runCommandInExec(config, buildArgs, r.cmd, r.output, r.buildSecrets)
}

func runCommandInExec(config *v1.Config, buildArgs *dockerfile.BuildArgs, cmdRun *instructions.RunCommand, output *RunOutput, buildSecrets []string) (err error) {
	if output == nil {
		output = &RunOutput{}
	}
	if output.Stdout == nil {
		output.Stdout = os.Stdout
	}
	if output.Stderr == nil {
		output.Stderr = os.Stderr
	}
	var newCommand []string
	if cmdRun.PrependShell {
		// This is the default shell on Linux
		var shell []string
		if len(config.Shell) > 0 {
			shell = config.Shell
		} else {
			shell = append(shell, "/bin/sh", "-c")
		}

		newCommand = append(shell, strings.Join(cmdRun.CmdLine, " "))
	} else {
		newCommand = cmdRun.CmdLine
		// Find and set absolute path of executable by setting PATH temporary
		replacementEnvs := buildArgs.ReplacementEnvs(config.Env)
		for _, v := range replacementEnvs {
			entry := strings.SplitN(v, "=", 2)
			if entry[0] != "PATH" {
				continue
			}
			oldPath := os.Getenv("PATH")
			defer os.Setenv("PATH", oldPath)
			os.Setenv("PATH", entry[1])
			path, err := exec.LookPath(newCommand[0])
			if err == nil {
				newCommand[0] = path
			}
		}
	}

	logrus.Infof("Cmd: %s", newCommand[0])
	logrus.Infof("Args: %s", newCommand[1:])

	cmd := exec.Command(newCommand[0], newCommand[1:]...)

	cmd.Dir = setWorkDirIfExists(config.WorkingDir)
	cmd.Stdout = output.Stdout
	cmd.Stderr = output.Stderr
	replacementEnvs := buildArgs.ReplacementEnvs(config.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	u := config.User
	userAndGroup := strings.Split(u, ":")
	userStr, err := util.ResolveEnvironmentReplacement(userAndGroup[0], replacementEnvs, false)
	if err != nil {
		return errors.Wrapf(err, "resolving user %s", userAndGroup[0])
	}

	// If specified, run the command as a specific user
	if userStr != "" {
		cmd.SysProcAttr.Credential, err = util.SyscallCredentials(userStr)
		if err != nil {
			return errors.Wrap(err, "credentials")
		}
	}

	env, err := addDefaultHOME(userStr, replacementEnvs)
	if err != nil {
		return errors.Wrap(err, "adding default HOME variable")
	}

	cmdRun.Expand(func(word string) (string, error) {
		// NOTE(SasSwart): This is a noop function. It's here to satisfy the buildkit parser.
		// Without this, the buildkit parser won't parse --mount flags for RUN directives.
		// Support for expansion in RUN directives deferred until its needed.
		// https://docs.docker.com/build/building/variables/
		return word, nil
	})

	buildSecretsMap := make(map[string]string)
	for _, s := range buildSecrets {
		secretName, secretValue, found := strings.Cut(s, "=")
		if !found {
			return fmt.Errorf("invalid secret %s", s)
		}
		buildSecretsMap[secretName] = secretValue
	}

	secretFileManager := fileCreatorCleaner{}
	defer func() {
		cleanupErr := secretFileManager.Clean()
		if err == nil {
			err = cleanupErr
		}
	}()

	mounts := instructions.GetMounts(cmdRun)
	for _, mount := range mounts {
		switch mount.Type {
		case instructions.MountTypeSecret:
			// Implemented as per:
			// https://docs.docker.com/reference/dockerfile/#run---mounttypesecret

			envName := mount.CacheID
			secret, secretSet := buildSecretsMap[envName]
			if !secretSet && mount.Required {
				return fmt.Errorf("required secret %s not found", mount.CacheID)
			}

			// If a target is specified, we write to the file specified by the target:
			// If no target is specified and no env is specified, we write to /run/secrets/<id>
			// If no target is specified and an env is specified, we set the env and don't write to file
			if mount.Env == nil || mount.Target != "" {
				targetFile := mount.Target
				if targetFile == "" {
					targetFile = filepath.Join(secretsDir, mount.CacheID)
				}
				if !filepath.IsAbs(targetFile) {
					targetFile = filepath.Join(config.WorkingDir, targetFile)
				}
				secretFileManager.MkdirAndWriteFile(targetFile, []byte(secret), 0700, 0600)
			}

			// We don't return in the block above, because its possible to have both a target and an env.
			// As such we need this guard clause or we risk getting a nil pointer below.
			if mount.Env == nil {
				continue
			}

			targetEnv := *mount.Env
			if targetEnv == "" {
				targetEnv = mount.CacheID
			}

			env = append(env, fmt.Sprintf("%s=%s", targetEnv, secret))
		// NOTE(SasSwart):
		// Buildkit v0.16.0 brought support for `RUN --mount` flags. Kaniko support for the mount
		// types below is deferred until its needed.
		// case instructions.MountTypeBind:
		// case instructions.MountTypeTmpfs:
		// case instructions.MountTypeCache:
		// case instructions.MountTypeSSH
		default:
			logrus.Warnf("Mount type %s is not supported", mount.Type)
		}
	}

	cmd.Env = env

	logrus.Infof("Running: %s", cmd.Args)
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting command")
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return errors.Wrap(err, "getting group id for process")
	}
	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "waiting for process to exit")
	}

	// it's not an error if there are no grandchildren
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err.Error() != "no such process" {
		return err
	}
	return nil
}

// fileCreatorCleaner keeps tracks of all files and directories that it created in the order that they were created.
// Once asked to clean up, it will remove all files and directories in the reverse order that they were created.
type fileCreatorCleaner struct {
	filesToClean []string
	dirsToClean  []string
}

func (s *fileCreatorCleaner) MkdirAndWriteFile(path string, data []byte, dirPerm, filePerm os.FileMode) error {
	dirPath := filepath.Dir(path)
	parentDirs := strings.Split(dirPath, string(os.PathSeparator))

	// Start at the root directory
	currentPath := string(os.PathSeparator)

	for _, nextDirDown := range parentDirs {
		if nextDirDown == "" {
			continue
		}
		// Traverse one level down
		currentPath = filepath.Join(currentPath, nextDirDown)

		if _, err := filesystem.FS.Stat(currentPath); errors.Is(err, os.ErrNotExist) {
			if err := filesystem.FS.Mkdir(currentPath, dirPerm); err != nil {
				return err
			}
			s.dirsToClean = append(s.dirsToClean, currentPath)
		}
	}

	// With all parent directories created, we can now create the actual secret file
	if err := filesystem.FS.WriteFile(path, []byte(data), 0600); err != nil {
		return errors.Wrap(err, "writing secret to file")
	}
	s.filesToClean = append(s.filesToClean, path)

	return nil
}

func (s *fileCreatorCleaner) Clean() error {
	for i := len(s.filesToClean) - 1; i >= 0; i-- {
		if err := filesystem.FS.Remove(s.filesToClean[i]); err != nil {
			return err
		}
	}

	for i := len(s.dirsToClean) - 1; i >= 0; i-- {
		if err := filesystem.FS.Remove(s.dirsToClean[i]); err != nil {
			pathErr := new(fs.PathError)
			// If a path that we need to clean up is not empty, then that means
			// that a third party has placed something in there since we created it.
			// In that case, we should not remove it, because it no longer belongs exclusively to us.
			if errors.As(err, &pathErr) && pathErr.Err == syscall.ENOTEMPTY {
				continue
			}
			return err
		}
	}

	return nil
}

// addDefaultHOME adds the default value for HOME if it isn't already set
func addDefaultHOME(u string, envs []string) ([]string, error) {
	for _, env := range envs {
		split := strings.SplitN(env, "=", 2)
		if split[0] == constants.HOME {
			return envs, nil
		}
	}

	// If user isn't set, set default value of HOME
	if u == "" || u == constants.RootUser {
		return append(envs, fmt.Sprintf("%s=%s", constants.HOME, constants.DefaultHOMEValue)), nil
	}

	// If user is set to username, set value of HOME to /home/${user}
	// Otherwise the user is set to uid and HOME is /
	userObj, err := userLookup(u)
	if err != nil {
		return nil, fmt.Errorf("lookup user %v: %w", u, err)
	}

	return append(envs, fmt.Sprintf("%s=%s", constants.HOME, userObj.HomeDir)), nil
}

// String returns some information about the command for the image config
func (r *RunCommand) String() string {
	return r.cmd.String()
}

func (r *RunCommand) FilesToSnapshot() []string {
	return nil
}

func (r *RunCommand) ProvidesFilesToSnapshot() bool {
	return false
}

// CacheCommand returns true since this command should be cached
func (r *RunCommand) CacheCommand(img v1.Image) DockerCommand {
	return &CachingRunCommand{
		img:       img,
		cmd:       r.cmd,
		extractFn: util.ExtractFile,
	}
}

func (r *RunCommand) MetadataOnly() bool {
	return false
}

func (r *RunCommand) RequiresUnpackedFS() bool {
	return true
}

func (r *RunCommand) ShouldCacheOutput() bool {
	return r.shdCache
}

type CachingRunCommand struct {
	BaseCommand
	caching
	img            v1.Image
	extractedFiles []string
	cmd            *instructions.RunCommand
	extractFn      util.ExtractFunction
}

func (cr *CachingRunCommand) IsArgsEnvsRequiredInCache() bool {
	return true
}

func (cr *CachingRunCommand) ExecuteCommand(config *v1.Config, buildArgs *dockerfile.BuildArgs) error {
	logrus.Infof("Found cached layer, extracting to filesystem")
	var err error

	if cr.img == nil {
		return errors.New(fmt.Sprintf("command image is nil %v", cr.String()))
	}

	layers, err := cr.img.Layers()
	if err != nil {
		return errors.Wrap(err, "retrieving image layers")
	}

	if len(layers) != 1 {
		return errors.New(fmt.Sprintf("expected %d layers but got %d", 1, len(layers)))
	}

	cr.layer = layers[0]

	cr.extractedFiles, err = util.GetFSFromLayers(
		kConfig.RootDir,
		layers,
		util.ExtractFunc(cr.extractFn),
		util.IncludeWhiteout(),
	)
	if err != nil {
		return errors.Wrap(err, "extracting fs from image")
	}

	return nil
}

func (cr *CachingRunCommand) CachedExecuteCommand(config *v1.Config, buildArgs *dockerfile.BuildArgs) error {
	logrus.Infof("Found cached layer, faking extraction to filesystem")
	var err error

	if cr.img == nil {
		return errors.New(fmt.Sprintf("command image is nil %v", cr.String()))
	}

	layers, err := cr.img.Layers()
	if err != nil {
		return errors.Wrap(err, "retrieving image layers")
	}

	if len(layers) != 1 {
		return errors.New(fmt.Sprintf("expected %d layers but got %d", 1, len(layers)))
	}

	cr.layer = layers[0]
	cr.extractedFiles = []string{}

	return nil
}

func (cr *CachingRunCommand) FilesToSnapshot() []string {
	f := cr.extractedFiles
	logrus.Debugf("%d files extracted by caching run command", len(f))
	logrus.Tracef("Extracted files: %s", f)

	return f
}

func (cr *CachingRunCommand) String() string {
	if cr.cmd == nil {
		return "nil command"
	}
	return cr.cmd.String()
}

func (cr *CachingRunCommand) MetadataOnly() bool {
	return false
}

// todo: this should create the workdir if it doesn't exist, atleast this is what docker does
func setWorkDirIfExists(workdir string) string {
	if _, err := filesystem.FS.Lstat(workdir); err == nil {
		return workdir
	}
	return ""
}
