package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/web"
)

func ServeCMD(conf *config.Config, args []string) error {

	serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)

	hostF := serveFlags.String("host", "localhost", "web host to run server")
	portF := serveFlags.Uint("port", 6789, "web port for server to listen on")
	debugF := serveFlags.Bool("debug", false, "run server in debug mode")
	serveFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] web [subcommand options]\n\n")
		fmt.Fprint(os.Stderr, "\nOptions:\n")
		serveFlags.PrintDefaults()
		fmt.Fprint(os.Stderr, "\n\n")
	}

	err := serveFlags.Parse(args)
	if err != nil {
		return err
	}

	serveFlags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "host":
			conf.Web.Host = *hostF
		case "port":
			conf.Web.Port = *portF
		case "debug":
			conf.Web.Debug = *debugF
		}
	})

	return web.RunServer(conf)

}
