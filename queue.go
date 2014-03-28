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

import "sync"

type WorkspaceQueue struct {
	chans map[string]chan bool
	mu    *sync.Mutex
}

func newWorkspaceQueue() *WorkspaceQueue {
	return &WorkspaceQueue{
		chans: make(map[string]chan bool),
		mu:    new(sync.Mutex),
	}
}

func (queue *WorkspaceQueue) GetWorkspaceQueue(workspace string) chan bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	ch, ok := queue.chans[workspace]
	if ok {
		return ch
	}

	ch = make(chan bool, 1)
	queue.chans[workspace] = ch
	return ch
}

	// Make sure the workspace exists.
	if err := os.Stat(workspace); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(workspace, 0750); err != nil {
				request.Resolve(5, &data.BuildResult{Error: err.Error()})
				return
			}
		} else {
			request.Resolve(6, &data.BuildResult{Error: err.Error()})
			return
		}
	}


