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

package commands

import (
	"strings"
	"testing"

	"github.com/GoogleContainerTools/kaniko/pkg/util"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/linter"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
)

func parseInstruction(t *testing.T, line string) instructions.Command {
	t.Helper()
	ast, err := parser.Parse(strings.NewReader(line))
	if err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	inst, err := instructions.ParseInstructionWithLinter(ast.AST.Children[0], linter.New(&linter.Config{}))
	if err != nil {
		t.Fatalf("parse instruction %q: %v", line, err)
	}
	cmd, ok := inst.(instructions.Command)
	if !ok {
		t.Fatalf("%q parsed to %T, want a command", line, inst)
	}
	return cmd
}

// TestGetCommandRejectsUnsupportedFlags covers flags that buildkit v0.16
// rejected while parsing and newer buildkit accepts. kaniko does not
// implement them, so it must fail instead of silently ignoring them.
func TestGetCommandRejectsUnsupportedFlags(t *testing.T) {
	for _, tc := range []struct {
		line    string
		wantErr string
	}{
		{line: "COPY --exclude=*.secret . /app", wantErr: "--exclude"},
		{line: "ADD --exclude=*.secret . /app", wantErr: "--exclude"},
		{line: "COPY --parents a/b/c /app/", wantErr: "--parents"},
		{line: "ADD --unpack=false archive.tar /app/", wantErr: "--unpack"},
		{line: "ADD --unpack=true archive.tar /app/", wantErr: "--unpack"},
		{line: "RUN --security=insecure echo hi", wantErr: "--security=insecure"},
		{line: "RUN --device=/dev/fuse echo hi", wantErr: "--device"},
		{line: "COPY . /app"},
		{line: "ADD archive.tar /app/"},
		{line: "RUN echo hi"},
		{line: "RUN --security=sandbox echo hi"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			for _, useNewRun := range []bool{false, true} {
				_, err := GetCommand(parseInstruction(t, tc.line), util.FileContext{}, useNewRun, false, false, nil, nil)
				if tc.wantErr == "" {
					if err != nil {
						t.Fatalf("GetCommand(useNewRun=%t): %v", useNewRun, err)
					}
					continue
				}
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("GetCommand(useNewRun=%t) error = %v, want one mentioning %q", useNewRun, err, tc.wantErr)
				}
			}
		})
	}
}

func TestGetCommandAcceptsRunCommandWithoutParserState(t *testing.T) {
	// RunCommand values built directly, as in tests, carry no parser state.
	cmd := &instructions.RunCommand{ShellDependantCmdLine: instructions.ShellDependantCmdLine{CmdLine: []string{"echo hi"}}}
	if _, err := GetCommand(cmd, util.FileContext{}, false, false, false, nil, nil); err != nil {
		t.Fatalf("GetCommand: %v", err)
	}
}
