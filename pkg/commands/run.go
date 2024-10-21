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
	cmd      *instructions.RunCommand
	output   *RunOutput
	secrets  []string
	shdCache bool
}

// for testing
var (
	userLookup = util.LookupUser
)

func (r *RunCommand) IsArgsEnvsRequiredInCache() bool {
	return true
}

func (r *RunCommand) ExecuteCommand(config *v1.Config, buildArgs *dockerfile.BuildArgs) error {
	return runCommandInExec(config, buildArgs, r.cmd, r.output, r.secrets)
}

func runCommandInExec(config *v1.Config, buildArgs *dockerfile.BuildArgs, cmdRun *instructions.RunCommand, output *RunOutput, secrets []string) error {
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

	// TODO (sas): figure out what kind of expansion we need to do to be standards compliant.
	// Right now we don't do any kind of expansions, but parseMount() requires an expander, even
	// if its a noop
	cmdRun.Expand(func(word string) (string, error) {
		return word, nil
	})

	var secretFilesToClean []string
	mounts := instructions.GetMounts(cmdRun)
	for _, mount := range mounts {
		switch mount.Type {
		case instructions.MountTypeSecret:
			// Implemented as per:
			// https://docs.docker.com/reference/dockerfile/#run---mounttypesecret

			// TODO (sas): turn secrets into either a map outside the loop so it can be accessed efficiently
			var secret string
			secretSet := false
			envName := mount.CacheID
			for _, s := range secrets {
				parts := strings.SplitN(s, "=", 2)
				if len(parts) == 2 && parts[0] == envName {
					secret = parts[1]
					secretSet = true
					break
				}
			}
			if !secretSet && mount.Required {
				return fmt.Errorf("required secret %s not found", mount.CacheID)
			}

			// If a target is specified, we write to the file specified by the target:
			// If no target is specified and no env is specified, we write to /run/secrets/<id>
			// If no target is specified and an env is specified, we set the env and don't write to file
			if mount.Env == nil || mount.Target != "" {
				targetFile := mount.Target
				if targetFile == "" {
					targetFile = fmt.Sprintf("/run/secrets/%s", mount.CacheID)
				}

				// TODO (sas): check what these file modes should be set to.
				os.MkdirAll(filepath.Dir(targetFile), 0700)
				if err := os.WriteFile(targetFile, []byte(secret), 0600); err != nil {
					return errors.Wrap(err, "writing secret to file")
				}
				secretFilesToClean = append(secretFilesToClean, targetFile)
			}

			if mount.Env == nil {
				continue
			}

			targetEnv := *mount.Env
			if targetEnv == "" {
				targetEnv = mount.CacheID
			}

			env = append(env, fmt.Sprintf("%s=%s", targetEnv, secret))
		// Nice to haves beyond the scope of the current issue:
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

	for _, secretFile := range secretFilesToClean {
		if err := os.Remove(secretFile); err != nil {
			return errors.Wrap(err, "removing secret file")
		}
	}

	// it's not an error if there are no grandchildren
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err.Error() != "no such process" {
		return err
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
