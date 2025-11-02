package main

import (
	"flag"
	"fmt"
	"os"
)

func runServer(args []string) error {
	return nil

}

func usage() {
	fmt.Fprintln(os.Stdout, "communis:")
	fmt.Fprintln(os.Stdout, "    server    run web server")
	fmt.Fprintln(os.Stdout, "    database  manage database")
}

func main() {

	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(0)
	}

	var err error
	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "server":
		err = runServer(subArgs)
	case "database":
		err = runDatabase(subArgs)
	default:
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s is not a valid command", cmd))
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

}
