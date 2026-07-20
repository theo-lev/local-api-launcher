package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	defaultManagerPort = 9000
	lastFallbackPort   = 9010
	managerPortEnv     = "API_MANAGER_PORT"
)

type startupOptions struct {
	port     int
	explicit bool
}

func parseStartupOptions(args []string, getenv func(string) string, output io.Writer) (startupOptions, error) {
	flags := flag.NewFlagSet("api-manager", flag.ContinueOnError)
	flags.SetOutput(output)
	portFlag := flags.String("port", "", "manager HTTP port (overrides API_MANAGER_PORT; default 9000 with automatic fallback through 9010)")
	if err := flags.Parse(args); err != nil {
		return startupOptions{}, err
	}
	if flags.NArg() != 0 {
		return startupOptions{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	flagProvided := false
	flags.Visit(func(option *flag.Flag) {
		flagProvided = flagProvided || option.Name == "port"
	})
	value := strings.TrimSpace(*portFlag)
	explicit := flagProvided
	if !flagProvided {
		value = strings.TrimSpace(getenv(managerPortEnv))
		explicit = value != ""
	}
	if !explicit {
		return startupOptions{port: defaultManagerPort}, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return startupOptions{}, fmt.Errorf("invalid manager port %q: must be a number from 1 to 65535", value)
	}
	return startupOptions{port: port, explicit: true}, nil
}

type listenFunc func(network, address string) (net.Listener, error)

func listenForStartup(options startupOptions, listen listenFunc) (net.Listener, int, error) {
	if options.explicit {
		listener, err := listen("tcp", loopbackAddress(options.port))
		if err != nil {
			return nil, 0, fmt.Errorf("configured port %d is unavailable: %w; choose another with --port", options.port, err)
		}
		return listener, options.port, nil
	}

	var listenErrors []error
	for port := defaultManagerPort; port <= lastFallbackPort; port++ {
		listener, err := listen("tcp", loopbackAddress(port))
		if err == nil {
			return listener, port, nil
		}
		listenErrors = append(listenErrors, err)
	}
	return nil, 0, fmt.Errorf("no available manager port in %d-%d: %w; choose a port with --port", defaultManagerPort, lastFallbackPort, errors.Join(listenErrors...))
}

func loopbackAddress(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}
