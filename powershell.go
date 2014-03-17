// Copyright (c) 2014 Salsita s.r.o.
//
// This file is part of paprika-slave.
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
	"os"
	"os/exec"

	"github.com/cider/ciderd/apps/util/executil"
	"github.com/cider/ciderd/apps/util/vcsutil"
	"github.com/cider/go-cider/cider/clients/rpc"

	"github.com/salsita-cider/cider-buildslave-win/data"
)

func (slave *BuildSlave) RunPowerShellScript(request rpc.RemoteRequest) {
	// Unmarshal the arguments.
	var args data.PSArgs
	if err := request.UnmarshalArgs(&args); err != nil {
		request.Resolve(3, data.PSReply{Error: err.Error()})
		return
	}

	// Make sure that all required arguments are set.
	if err := args.Validate(); err != nil {
		request.Resolve(4, data.PSReply{Error: err.Error()})
		return
	}

	// Clone or pull the project sources.
	srcDir := slave.workspaceForProject(args.ProjectUsername, args.ProjectReponame)
	srcExists := true
	if err := os.Stat(srcDir); err != nil {
		if os.IsNotExist(err) {
			srcExists = false
		} else {
			request.Resolve(5, data.PSReply{Error: err.Error()})
			return
		}
	}

	if !srcExists {
		if err := os.MkdirAll(srcDir, 0750); err != nil {
			request.Resolve(6, data.PSReply{Error: err.Error()})
			return
		}
	}

	repo := fmt.Sprintf("github.com/%s/%s", args.ProjectUsername, args.ProjectReponame)
	repoURL := url.Parse("git+ssh://" + repo)

	git, _ := vcsutil.NewVCS("git+ssh")
	var err error
	if srcExists {
		err = git.Pull(repoURL, srcDir, request)
	} else {
		err = git.Clone(repoURL, srcDir, request)
	}
	if err != nil {
		request.Resolve(7, data.PSReply{Error: err.Error()})
		return
	}

	// Prepare the command to be executed.
	cmd := exec.Command("PowerShell", "-NoLogo", "-File", args.Filename)
	cmd.Stdout = request.Stdout()
	cmd.Stderr = request.Stderr()

	if os.Getenv("WORKSPACE") != "" {
		request.Resolve(8, data.PSReply{Error: "WORKSPACE variable already set"})
		return
	}

	env := os.Environ()
	env = append(env, "WORKSPACE="+srcDir)
	cmd.Env = env

	// Execute PowerShell.
	if err := executil.Run(cmd, request.Interrupted()); err != nil {
		request.Resolve(9, data.PSReply{Error: err.Error()})
		return
	}

	// Resolve the request.
	request.Resolve(0, data.PSReply{})
}
