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
	// Stdlib
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	// Paprika
	"github.com/salsita-cider/paprika-slave/runners"

	// Cider
	"github.com/cider/go-cider/cider/services/rpc"
	ws "github.com/cider/go-cider/cider/transports/websocket/rpc"

	// Others
	"code.google.com/p/go.net/websocket"
)

const TokenHeader = "X-Paprika-Token"

func main() {
	log.SetFlags(0)

	// Parse the command line.
	fidentity := flag.String("identity", "", "build slave unique identity")
	fmaster := flag.String("master", "", "master node to connect to")
	ftoken := flag.String("token", "", "master node access token")
	flabels := flag.String("labels", "", "labels to apply to this build slave")
	fexecutors := flag.Int("executors", runtime.NumCPU(), "number of executors")
	fworkspace := flag.String("workspace", "", "build workspace")

	// Parse the environment.
	getenvOrFailNow(fidentity, "PAPRIKA_IDENTITY", "")
	getenvOrFailNow(fmaster, "PAPRIKA_MASTER", "")
	getenvOrFailNow(ftoken, "PAPRIKA_TOKEN", "")
	getenvOrFailNow(flabels, "PAPRIKA_LABELS", "")
	getenvOrFailNow(fworkspace, "PAPRIKA_WORKSPACE", "")

	// Start catching signals.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Connect to the master node using the WebSocket transport.
	// The specified token is used to authenticated the build slave.
	srv, err := rpc.NewService(func() (rpc.Transport, error) {
		factory := ws.NewTransportFactory()
		factory.Server = *fmaster
		factory.WSConfigFunc = func(config *websocket.Config) {
			config.Header.Set(TokenHeader, *ftoken)
		}
		return factory.NewTransport(*fidentity)
	})
	if err != nil {
		log.Fatal(err)
	}

	// Number of concurrent builds is limited by creating a channel of the
	// specified length. Every time a build is requested, the request handler
	// sends some data to the channel, and when it is finished, it reads data
	// from the same channel.
	execQueue := make(chan bool, *fexecutors)

	// Export all available runners.
	fmt.Println("Available runners:")
	for _, runner := range runners.Available {
		log.Printf("  %v\n", runner.Name)
	}

	manager := newWorkspaceManager(*fworkspace)

	for _, label := range strings.Split(*flabels, ",") {
		for _, runner := range runners.Available {
			methodName := label + "." + runner.Name
			builder := &Builder{runner, manager, execQueue}
			srv.MustRegister(methodName, builder.Build)
		}
	}

	// Block until either there is an error or a signal is received.
	select {
	case <-srv.Closed():
	case <-signalCh:
		if err := srv.Close(); err != nil {
			log.Fatal(err)
		}
	}

	if err := srv.Wait(); err != nil {
		log.Fatal(err)
	}
}

func getenvOrFailNow(value *string, key string, defaultValue string) {
	// In case the flag was used, we do not read the environment.
	if *value != defaultValue {
		return
	}

	// Read the value from the environment.
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Error: %v is not set and neither is the relevant flag", key)
	}

	*value = v
}
