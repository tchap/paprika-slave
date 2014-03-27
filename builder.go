// Copyright (c) 2014 Salsita s.r.o.
//
// This file is part of paprika-slave.
//
// paprika-slave is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// paprika-slave is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with paprika-slave.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"

	"github.com/salsita-cider/paprika/data"

	"github.com/cider/go-cider/services/rpc"
	"github.com/salsita-cider/paprika-slave/runners"
)

type Builder struct {
	runner     *runners.Runner
	workspace  string
	executorCh chan bool
}

func (builder *Builder) Build(request rpc.RemoteRequest) {
	// Some shortcuts.
	stdout := request.Stdout()
	stderr := request.Stderr()

	// Unmarshal and validate the input data.
	var args data.BuildArgs
	if err := request.UnmarshalArguments(&args); err != nil {
		request.Resolve(2, &data.BuildResult{Error: err})
		return
	}
	if err := args.Validate(); err != nil {
		request.Resolve(3, &data.BuildResult{Error: err})
		return
	}

	// Wait for a free executor.
	stdout.Write("---> Waiting for a free executor\n")
	for {
		select {
		case builder.executorCh <- true:
			stdout.Write("---> Executor acquired, starting the build\n")
		case <-request.Interrupted():
			stdout.Write("---> Build interrupted\n")
			request.Resolve(4, &data.BuildResult{Error: ErrInterrupted})
			return
		case <-time.After(20 * time.Second):
			request.Stdout.Write("---> ...\n")
		}
	}
	// Release the executor on return.
	defer func() {
		<-builder.executorCh
		stdout.Write("---> Executor released\n")
	}()

	// Make sure the workspace exists.

	// Check out the sources at the right revision.

	// Run the specified script.
}
