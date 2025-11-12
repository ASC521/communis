package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/web"
)

func WebCMD(conf *config.Config, args []string) error {

	runCMD := func(conf *config.Config, args []string) error {

		runFlags := flag.NewFlagSet("web-run", flag.ExitOnError)
		hostF := runFlags.String("host", "localhost", "web host to run server")
		portF := runFlags.Uint("port", 6789, "web port for server to listen on")

		runFlags.Usage = func() {
			fmt.Fprint(os.Stdout, "Usage: communis [global options] web run [subcommand options]\n\n")
			fmt.Fprint(os.Stdout, "Options:\n")
			runFlags.PrintDefaults()
			fmt.Fprint(os.Stdout, "\n\n")
		}

		err := runFlags.Parse(args)
		if err != nil {
			return err
		}

		runFlags.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "host":
				conf.Web.Host = *hostF
			case "port":
				conf.Web.Port = *portF
			}
		})

		if hostF != nil {
			conf.Web.Host = *hostF
		}
		if portF != nil {
			conf.Web.Port = *portF
		}

		return web.RunServer(conf)
	}

	webFlags := flag.NewFlagSet("web", flag.ExitOnError)
	webFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] web <subcommand>\n\n")
		fmt.Fprint(os.Stdout, "\nAvailable Commands:\n")
		fmt.Fprint(os.Stdout, "run     starts web server\n\n")
	}

	err := webFlags.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		webFlags.Usage()
		return nil
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "run":
		return runCMD(conf, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "command %s is not supported\n", cmd)
		os.Exit(1)
	}

	return nil

}
