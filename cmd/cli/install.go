package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/ASC521/communis/config"
)

func InstallCMD(conf *config.Config, args []string) error {

	installFlags := flag.NewFlagSet("install", flag.ExitOnError)
	usernameF := installFlags.String("username", "communis", "system user name")
	systemdF := installFlags.Bool("systemd", true, "generate systemd unit file")
	systemdDirF := installFlags.String("systemd-dir", "/etc/systemd/system", "directory to write systemd unit file")
	installFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] install [install options]\n\n")
		fmt.Fprint(os.Stdout, "Options:\n")
		installFlags.PrintDefaults()
		fmt.Fprint(os.Stdout, "\n\n")
	}
	installFlags.Parse(args)

	if runtime.GOOS != config.LinuxOS {
		return fmt.Errorf("%s operating system is not support for automated installation", runtime.GOOS)
	}

	if os.Getuid() != 0 {
		return errors.New("install command must be run as root user")
	}

	svcUser, err := user.Lookup(*usernameF)
	if err != nil {
		return fmt.Errorf("service user %s does not exist - create user first: %w", *usernameF, err)
	}

	uid, _ := strconv.Atoi(svcUser.Uid)
	gid, _ := strconv.Atoi(svcUser.Gid)

	dirsToMake := []string{
		conf.DataDirectory,
		filepath.Dir(conf.FileLocation),
	}

	for _, dir := range dirsToMake {
		if err = os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
		if err = os.Chown(dir, uid, gid); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ %s created (owned by %s)\n", dir, svcUser.Username)
	}

	if *systemdF {
		err = generateSystemdUnitFile(*systemdDirF, *usernameF, dirsToMake)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ communis.service created @ %s\n", *systemdDirF)
	}

	err = generateConfig(conf, conf.FileLocation)
	if err != nil {
		return err
	}
	if err = os.Chown(conf.FileLocation, uid, gid); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "✓ config.toml created @ %s (owned by %s)\n", conf.FileLocation, svcUser.Username)
	return nil
}

func UninstallCMD(conf *config.Config, args []string) error {
	uninstallFlags := flag.NewFlagSet("uninstall", flag.ExitOnError)
	removeDataF := uninstallFlags.Bool("remove-data", true, "remove communis notes databases")
	systemdDirF := uninstallFlags.String("systemd-dir", "/etc/systemd/system/", "directory containing systemd service file")

	uninstallFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] uninstall [uninstall options]\n\n")
		fmt.Fprint(os.Stdout, "Options:\n")
		uninstallFlags.PrintDefaults()
		fmt.Fprint(os.Stdout, "\n\n")
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
	fmt.Fprintf(os.Stdout, "✓ %s deleted\n", unitFile)

	// Remove Data
	if *removeDataF {
		err = os.RemoveAll(conf.DataDirectory)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ %s deleted\n", conf.DataDirectory)
	}

	// Remove Config location
	err = os.RemoveAll(filepath.Dir(conf.FileLocation))
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ %s deleted\n", filepath.Dir(conf.FileLocation))

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
	fmt.Fprintf(os.Stdout, "✓ %s deleted\n", exe)

	return nil

}
