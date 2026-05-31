package main

import (
	"errors"
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

	if conf.WebEnableHTTPS {
		if conf.WebCert == "" {
			return errors.New("certificate file is required to run an HTTPS server")
		}

		if _, err := os.Stat(conf.WebCert); os.IsNotExist(err) {
			return errors.New("certificate file does not exist")
		}

		if conf.WebKey == "" {
			return errors.New("key file is required to run an HTTPS server")
		}

		if _, err := os.Stat(conf.WebKey); os.IsNotExist(err) {
			return errors.New("key file does not exist")
		}

	}

	return web.RunServer(conf)

}
