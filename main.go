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
	// Stdlib
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	// Cider
	"github.com/cider/go-cider/cider/services/rpc"
	ws "github.com/cider/go-cider/cider/transports/websocket/rpc"

	// Others
	"code.google.com/p/go.net/websocket"
)

const TokenHeader = "X-Paprika-Token"

// This map contains all the script runners that are to be registered at slave
// startup. The content will vary depending on the platform. For example, PowerShell
// will not be available on Linux or Mac OS X, naturally.
var runners = make(map[string]rpc.RequestHandler)

func main() {
	log.SetFlags(0)

	// Parse the command line.
	fidentity := flag.String("identity", "", "build slave unique identity")
	fmaster := flag.String("master", "", "master node to connect to")
	ftoken := flag.String("token", "", "master node access token")
	ftags := flag.String("tags", "", "tags to apply to this build slave")
	fexecutors := flag.Int("executors", runtime.NumCPU(), "number of executors")
	fworkspace := flag.String("workspace", "", "build workspace")

	// Parse the environment.
	getenvOrFailNow(fidentity, "PAPRIKA_IDENTITY", "")
	getenvOrFailNow(fmaster, "PAPRIKA_MASTER", "")
	getenvOrFailNow(ftoken, "PAPRIKA_TOKEN", "")
	getenvOrFailNow(ftags, "PAPRIKA_TAGS", "")
	getenvOrFailNow(fworkspace, "PAPRIKA_WORKSPACE", "")

	// Start catching signals.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Connect to the master node.
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

	// Export relevant methods.
	tags := strings.Split(*ftags, ",")
	for runner, handler := range runners {
		for _, tag := range tags {
			method := tag + "." + runner
			srv.MustRegister(method, handler)
			log.Printf("--> method %v exported\n", method)
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
	if v := os.Getenv(key); v == "" {
		log.Fatalf("Error: %v is not set", key)
	} else {
		*value = v
	}
}
