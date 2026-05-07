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
		fmt.Fprint(os.Stdout, "Usage: communis [global options] generate <subcommand>\n\n")
		fmt.Fprint(os.Stdout, "Available Commands:\n")
		fmt.Fprint(os.Stdout, "css\n")
		fmt.Fprint(os.Stdout, "config\n")
		fmt.Fprint(os.Stdout, "systemd-unit\n\n")
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
		return configCMD(conf, subArgs)
	case "systemd-unit":
		return systemdUnitFileCMD(conf, subArgs)
	default:
		return fmt.Errorf("%s is not a valid command", cmd)
	}

}

func generateCSS(args []string) error {

	cssFlags := flag.NewFlagSet("css", flag.ExitOnError)
	darkThemeF := cssFlags.String("dark-theme", "", "name of dark chroma theme")
	lightThemeF := cssFlags.String("light-theme", "", "name of light chroma theme")
	cssFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: communis [global options] generate-css [subcommand options]\n\n")
		fmt.Fprint(os.Stdout, "Options:\n")
		cssFlags.PrintDefaults()
		fmt.Fprint(os.Stdout, "\n\n")
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
		fmt.Fprintf(os.Stdout, "generated css file at %s\n", fn)

		return nil
	}

	err = writeCSS(darkStyle, "dark")
	if err != nil {
		return err
	}
	return writeCSS(lightStyle, "light")
}

func configCMD(conf *config.Config, args []string) error {
	configFlags := flag.NewFlagSet("config", flag.ExitOnError)
	outF := configFlags.String("out", "", "directory to write config file")

	err := configFlags.Parse(args)
	if err != nil {
		return err
	}

	outDir := ""
	configFlags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "out":
			outDir = *outF
		}
	})

	if outDir == "" {
		return errors.New("out cannot be empty")
	}

	return generateConfig(conf, outDir)

}

func generateConfig(conf *config.Config, fileLoc string) error {

	confLoc, err := homedir.Expand(fileLoc)
	if err != nil {
		return err
	}
	confLoc, err = filepath.Abs(confLoc)
	if err != nil {
		return err
	}

	b, err := toml.Marshal(*conf)
	if err != nil {
		return err
	}

	return os.WriteFile(confLoc, b, 0700)

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
	unitFlags := flag.NewFlagSet("unit-file", flag.ExitOnError)
	userF := unitFlags.String("user", "communis", "name of user")
	outF := unitFlags.String("out", "/etc/systemd/system", "directory to write unit file")

	err := unitFlags.Parse(args)
	if err != nil {
		return err
	}

	return generateSystemdUnitFile(*outF, *userF, []string{conf.DataDirectory})

}

func generateSystemdUnitFile(outDirectory, username string, readWritePaths []string) error {

	out, err := homedir.Expand(outDirectory)
	if err != nil {
		return err
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}

	userP, err := user.Lookup(username)
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
		ReadWritePaths: readWritePaths,
	}

	outFile := filepath.Join(out, "communis.service")
	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return err
	}
	b := new(bytes.Buffer)
	err = tmpl.Execute(b, opts)
	if err != nil {
		return err
	}

	return os.WriteFile(outFile, b.Bytes(), 0700)
}
