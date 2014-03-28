// Copyright (c) 2014 Salsita s.r.o.
//
// This file is part of paprika.
//
// paprika is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// paprika is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with paprika.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"

	"github.com/salsita-cider/paprika/data"

	"github.com/cider/go-cider/services/rpc"
	"github.com/salsita-cider/paprika-slave/runners"
)

type Builder struct {
	runner    *runners.Runner
	wsManager *WorkspaceManager
	execQueue chan bool
}

func (builder *Builder) Build(request rpc.RemoteRequest) {
	// Some shortcuts.
	stdout := request.Stdout()
	stderr := request.Stderr()

	// Unmarshal and validate the input data.
	var args data.BuildArgs
	if err := request.UnmarshalArguments(&args); err != nil {
		request.Resolve(2, &data.BuildResult{Error: err.Error()})
		return
	}
	if err := args.Validate(); err != nil {
		request.Resolve(3, &data.BuildResult{Error: err.Error()})
		return
	}

	// Generate the project workspace and make sure it exists.
	repoURL, _ := url.Parse(args.Repository)
	workspace, err := builder.wsManager.EnsureWorkspace(repoURL)
	if err != nil {
		request.Resolve(4, &data.BuildResult{Error: err.Error()})
		return
	}

	// Acquire the workspace lock.
	wsQueue := builder.wsManager.WorkspaceQueue(workspace)
	if err := acquire("the workspace lock", wsQueue, request); err != nil {
		request.Resolve(5, &data.BuildResult{Error: err})
		return
	}
	defer release("the workspace lock", wsQueue, request)

	// Acquire a build executor.
	if err := acquire("a build executor", builder.execQueue, request); err != nil {
		request.Resolve(5, &data.BuildResult{Error: err})
		return
	}
	defer release("the build executor", builder.execQueue, request)

	// Start measuring the build time.
	startTimestamp := time.Now()

	// Check out the sources at the right revision.
	var (
		srcDir       = builder.wsManager.SrcDir(workspace)
		srcDirExists = builder.wsManager.SrcDirExists(workspace)
	)

	vcs, err := vcsutil.NewVCS(repoURL.Scheme)
	if err != nil {
		resolve(request, 6, startTimestamp, err)
		return
	}

	if srcDirExists {
		err = vcs.Pull(repoURL, srcDir, request)
	} else {
		err = vcs.Clone(repoURL, srcDir, request)
	}
	if err != nil {
		resolve(request, 7, startTimestamp, err)
		return
	}

	// Run the specified script.
	cmd := builder.runner.NewCommand(args.Script)

	env := os.Environ()
	env = append(env, args.Env...)
	env = append(env, "WORKSPACE="+workspace, "SRCDIR="+srcDir)
	cmd.Env = env

	cmd.Dir = srcDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	fmt.Fprintf(stdout, "---> Running the specified script: %v\n", args.Script)
	err = executil.Run(cmd, request.Interrupted())
	fmt.Fprintln(stdout, "---> Build finished")
	if err != nil {
		resolve(request, 1, startTimestamp, err)
		return
	}

	// Return success, at last.
	resolve(request, 0, startTimestamp, nil)
}

func resolve(req rpc.RemoteRequest, retCode byte, start time.Time, err error) {
	retValue := &data.BuildResult{
		Duration: time.Now().Sub(start),
	}
	if err != nil {
		retValue.Error = err.Error()
	}
	req.Resolve(retCode, retValue)
}

func ensureProjectWorkspace(workspace string, repoURL *net.URL) (ws string, err error) {
	// Generate the project workspace path from the global workspace and
	// the repository URL so that the same repository names do not collide
	// unless the whole repository URLs are the same.
	ws = filepath.Join(workspace, repoURL.Host, repoURL.Path)

	// Make sure the project workspace exists.
	err = os.Stat(ws)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(ws, 0750)
		}
	}
	return
}

func acquire(what string, queue chan bool, request rpc.RemoteRequest) (err string) {
	stdout := request.Stdout()
	fmt.Fprintf(stdout, "---> Waiting for %v\n", desc)
	for {
		select {
		case queue <- true:
			fmt.Fprintf(stdout, "---> Acquired %v", s)
			return
		case <-request.Interrupted():
			fmt.Fprintln(stdout, "---> Build interrupted")
			return "interrupted"
		case <-time.After(20 * time.Second):
			fmt.Fprintln(stdout, "---> ...")
		}
	}
}

func release(what string, queue chan bool, request rpc.RemoteRequest) {
	<-queue
	fmt.Fprintf(request.Stdout(), "---> Releasing %v\n", what)
}
