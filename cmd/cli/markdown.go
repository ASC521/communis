package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ASC521/communis/config"
	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/mitchellh/go-homedir"
)

func GenerateCssCMD(conf *config.Config, args []string) error {

	cssFlags := flag.NewFlagSet("css", flag.ExitOnError)
	darkThemeF := cssFlags.String("dark-theme", "", "name of dark chroma theme")
	lightThemeF := cssFlags.String("light-theme", "", "name of light chroma theme")
	outputDirF := cssFlags.String("output-dir", "../../web/static/vendor/css/", "directory to write css files")
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

	outputDir, err := homedir.Expand(*outputDirF)
	if err != nil {
		return fmt.Errorf("invalid output directory %s: %v", *outputDirF, err)
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output directory %s: %v", *outputDirF, err)
	}
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to make output directory: %v", err)
	}

	writeCss := func(s *chroma.Style, darkLight string) error {
		var buf bytes.Buffer
		formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.ClassPrefix("renderedmd-"))
		if err := formatter.WriteCSS(&buf, s); err != nil {
			return fmt.Errorf("failed to generate css theme %s: %v\n", s.Name, err)
		}

		fn := filepath.Join(outputDir, fmt.Sprintf("highlighting-%s.css", darkLight))
		if err = os.WriteFile(fn, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write theme file: %v", err)
		}
		fmt.Fprintf(os.Stdout, "generated css file at %s\n", fn)

		return nil
	}

	err = writeCss(darkStyle, "dark")
	if err != nil {
		return err
	}
	return writeCss(lightStyle, "light")
}
