package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ASC521/communis/config"
)

func UninstallCMD(conf *config.Config, args []string) error {
	uninstallFlags := flag.NewFlagSet("uninstall", flag.ExitOnError)
	removeDataF := uninstallFlags.Bool("remove-data", true, "remove communis notes databases")
	systemdDirF := uninstallFlags.String("systemd-dir", "/etc/systemd/system/", "directory containing systemd service file")

	uninstallFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] uninstall [uninstall options]\n\n")
		fmt.Fprint(os.Stderr, "Options:\n")
		uninstallFlags.PrintDefaults()
		fmt.Fprint(os.Stderr, "\n\n")
	}

	err := uninstallFlags.Parse(args)
	if err != nil {
		return err
	}

	if runtime.GOOS != config.LinuxOS {
		return fmt.Errorf("%s operating system is not support for automated uninstallation", runtime.GOOS)
	}

	if os.Getuid() != 0 {
		return errors.New("uninstall command must be run as root user")
	}

	unitFile := filepath.Join(*systemdDirF, fmt.Sprintf("%s.service", config.AppName))
	err = os.Remove(unitFile)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ %s deleted\n", unitFile)

	// Remove Data
	if *removeDataF {
		err = os.RemoveAll(conf.DataDirectory)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "✓ %s deleted\n", conf.DataDirectory)
	}

	// Remove Config location
	err = os.RemoveAll(filepath.Dir(conf.FileLocation))
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ %s deleted\n", filepath.Dir(conf.FileLocation))

	// Remove binary
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	if err = os.Remove(exe); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ %s deleted\n", exe)

	return nil

}
