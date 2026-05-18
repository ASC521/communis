package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"text/template"

	"github.com/ASC521/communis/config"
	"github.com/BurntSushi/toml"
	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/mitchellh/go-homedir"
)

func GenerateCMD(conf *config.Config, args []string) error {

	generateFlags := flag.NewFlagSet("generate", flag.ExitOnError)
	generateFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] generate <subcommand>\n\n")
		fmt.Fprint(os.Stderr, "Available Commands:\n")
		fmt.Fprint(os.Stderr, "css\n")
		fmt.Fprint(os.Stderr, "config\n")
		fmt.Fprint(os.Stderr, "systemd-unit\n")
		fmt.Fprint(os.Stderr, "systemd-container\n\n")
	}
	err := generateFlags.Parse(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		generateFlags.Usage()
		return nil
	}

	cmd, subArgs := args[0], args[1:]
	switch cmd {
	case "css":
		return generateCSS(subArgs)
	case "config":

		b, err := toml.Marshal(*conf)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, string(b))
		return err

	case "systemd-unit":
		return systemdUnitFileCMD(conf, subArgs)
	case "systemd-container":
		return generateSystemdContainerFile(subArgs)
	default:
		return fmt.Errorf("%s is not a valid command", cmd)
	}

}

func generateCSS(args []string) error {

	cssFlags := flag.NewFlagSet("css", flag.ExitOnError)
	darkThemeF := cssFlags.String("dark-theme", "", "name of dark chroma theme")
	lightThemeF := cssFlags.String("light-theme", "", "name of light chroma theme")
	cssFlags.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: communis [global options] generate-css [subcommand options]\n\n")
		fmt.Fprint(os.Stderr, "Options:\n")
		cssFlags.PrintDefaults()
		fmt.Fprint(os.Stderr, "\n\n")
	}
	err := cssFlags.Parse(args)
	if err != nil {
		return err
	}

	if *darkThemeF == "" {
		return errors.New("dark-theme is required")
	}
	if *lightThemeF == "" {
		return errors.New("light-theme is required")
	}

	darkStyle, ok := styles.Registry[*darkThemeF]
	if !ok {
		return fmt.Errorf("dark-theme %s is not supported", *darkThemeF)
	}
	lightStyle, ok := styles.Registry[*lightThemeF]
	if !ok {
		return fmt.Errorf("light-theme %s is not supported", *lightThemeF)
	}

	relOutDir := "../../web/static/vendor/css/"
	outputDir, err := homedir.Expand(relOutDir)
	if err != nil {
		return fmt.Errorf("invalid output directory %s: %v", relOutDir, err)
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output directory %s: %v", relOutDir, err)
	}
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to make output directory: %v", err)
	}

	writeCSS := func(s *chroma.Style, darkLight string) error {
		var buf bytes.Buffer
		formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.ClassPrefix("renderedmd-"))
		if err := formatter.WriteCSS(&buf, s); err != nil {
			return fmt.Errorf("failed to generate css theme %s: %v", s.Name, err)
		}

		fn := filepath.Join(outputDir, fmt.Sprintf("highlighting-%s.css", darkLight))
		if err = os.WriteFile(fn, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write theme file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "generated css file at %s\n", fn)

		return nil
	}

	err = writeCSS(darkStyle, "dark")
	if err != nil {
		return err
	}
	return writeCSS(lightStyle, "light")
}

const unitTemplate = `[Unit]
Description=Communis Note Taking Server
Documentation=https://github.com/ASC521/communis
After=network.target

[Service]
Type=simple
User={{ .User }}
Group={{ .Group }}
ExecStart={{ .ExecPath }} serve
Restart=on-failure
RestartSec=5s

ReadWritePaths={{ range .ReadWritePaths }} {{ . }} {{ end }}

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target`

type UnitFileOptions struct {
	User           string
	Group          string
	ExecPath       string
	ReadWritePaths []string
}

func systemdUnitFileCMD(conf *config.Config, args []string) error {

	cu, err := user.Current()
	if err != nil {
		return err
	}

	unitFlags := flag.NewFlagSet("unit-file", flag.ExitOnError)
	userF := unitFlags.String("username", cu.Username, "name of user")

	err = unitFlags.Parse(args)
	if err != nil {
		return err
	}

	userP, err := user.Lookup(*userF)
	if err != nil {
		return err
	}

	group, err := user.LookupGroupId(userP.Gid)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	opts := UnitFileOptions{
		User:           userP.Username,
		Group:          group.Name,
		ExecPath:       exe,
		ReadWritePaths: []string{conf.DataDirectory, filepath.Dir(conf.FileLocation)},
	}

	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return err
	}
	b := new(bytes.Buffer)
	err = tmpl.Execute(b, opts)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(os.Stdout, b.String())
	return err

}

const containerTemplate = `[Unit]
Description=Communis Note Taking Server
Documentation=https://github.com/ASC521/communis
After=network.target

[Container]
Image=ghcr.io/asc521/communis:{{ .Version }}
Volume=communis-data:/var/opt/communis
Volume={{ .UserConfigLoc }}:/etc/opt/communis:Z,ro
PublishPort=6789:6789
Exec=serve

[Service]
Restart=on-failure

[Install]
WantedBy=default.target`

type containerOptions struct {
	UserConfigLoc string
	Version       string
}

func generateSystemdContainerFile(args []string) error {
	containerFlags := flag.NewFlagSet("container", flag.ExitOnError)
	usrConfigLocF := containerFlags.String("config-dir", "%h/.config/communis", "location of config file on user computer")
	err := containerFlags.Parse(args)
	if err != nil {
		return err
	}

	opts := containerOptions{
		UserConfigLoc: *usrConfigLocF,
		Version:       containerVersion,
	}

	tmpl, err := template.New("container").Parse(containerTemplate)
	if err != nil {
		return err
	}
	b := new(bytes.Buffer)
	err = tmpl.Execute(b, opts)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, b.String())
	return err
}
