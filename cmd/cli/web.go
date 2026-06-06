package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ASC521/communis/config"
	"github.com/ASC521/communis/slogx"
	userstore "github.com/ASC521/communis/user-store"
	"github.com/ASC521/communis/web"
)

func ServeCMD(conf *config.Config, args []string) error {

	serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)

	hostF := serveFlags.String("host", "localhost", "web host to run server")
	portF := serveFlags.Uint("port", 6789, "web port for server to listen on")
	httpsEnableF := serveFlags.Bool("https", false, "run server requiring https")
	certF := serveFlags.String("cert", "", "location of tls certificate file")
	keyF := serveFlags.String("key", "", "location of tls key file")
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
			conf.WebHost = *hostF
		case "port":
			conf.WebPort = *portF
		case "https":
			conf.WebEnableHTTPS = *httpsEnableF
		case "cert":
			conf.WebCert = *certF
		case "key":
			conf.WebKey = *keyF
		}
	})

	logger := slog.New(slogx.NewPipeHandler(os.Stderr, &slogx.HandlerOptions{Level: slog.LevelDebug, IncludeSource: false}))
	dsmConf, err := userstore.ConfigToSQLiteDataStoreConfig(conf)
	if err != nil {
		return err
	}
	dsm, err := userstore.NewSQLiteConnManager(dsmConf, logger)
	if err != nil {
		return err
	}

	svrConf, err := web.ConfigToServerConfig(conf)
	if err != nil {
		return err
	}

	return web.RunServer(svrConf, dsm, logger)

}
