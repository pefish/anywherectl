package main

import (
	"flag"
	"github.com/pefish/anywherectl/client"
	"github.com/pefish/anywherectl/internal/version"
	"github.com/pefish/anywherectl/listener"
	"github.com/pefish/anywherectl/server"
	go_logger "github.com/pefish/go-logger"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var subServer SubServerInterface
	secondArg := os.Args[1]
	if secondArg == "serve" {
		subServer = server.NewServer()
	} else if secondArg == "listen" {
		subServer = listener.NewListener(`test`)
	} else {
		subServer = client.NewClient()
	}

	flagSet := flag.NewFlagSet(version.AppName, flag.ExitOnError)

	flagSet.Bool("version", false, "print version string")

	flagSet.String("log-level", "info", "set log verbosity: debug, info, warn, or error")

	subServer.DecorateFlagSet(flagSet)
	subServer.ParseFlagSet(flagSet)

	logLevel := flagSet.Lookup("log-level").Value.(flag.Getter).Get().(string)
	go_logger.Logger = go_logger.NewLogger(go_logger.WithLevel(logLevel), go_logger.WithIsDebug(true))

	if flagSet.Lookup("version").Value.(flag.Getter).Get().(bool) {
		go_logger.Logger.Info(version.GetAppVersion(version.AppName))
		os.Exit(0)
	}

	exitChan := make(chan bool, 1)
	go func() {
		err := subServer.Start(flagSet)
		if err != nil {
			go_logger.Logger.Error(err)
		}
		close(exitChan)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signalChan:
	case <-exitChan:
	}

	go_logger.Logger.Info("Stopping...")
	subServer.Stop()
	go_logger.Logger.Info("Stopped")
}

type SubServerInterface interface {
	DecorateFlagSet(flagSet *flag.FlagSet)
	ParseFlagSet(flagset *flag.FlagSet)
	Start(flagSet *flag.FlagSet) error
	Stop()
}
