package melange

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"chainguard.dev/melange/pkg/renovate"
	"chainguard.dev/melange/pkg/renovate/bump"

	"chainguard.dev/melange/pkg/config"
	"github.com/pkg/errors"
)

type Packages struct {
	Config   config.Configuration
	Filename string
	Dir      string
	NoLint   []string
	Hash     string
}

type ConfigCheck struct {
	Package struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"package"`
}

func (c ConfigCheck) isMelangeConfig() bool {
	if c.Package.Name == "" {
		return false
	}
	if c.Package.Version == "" {
		return false
	}
	return true
}

// ReadPackageConfigs read the melange package config(s) from the target git repository so we can check if new versions exist
func ReadPackageConfigs(packageNames []string, dir string) (map[string]*Packages, error) {
	p := make(map[string]*Packages)

	// if package names were passed as CLI parameters load those packages
	if len(packageNames) > 0 {
		// get package by name
		for _, packageName := range packageNames {
			filename := packageName + ".yaml"
			fullPath := filepath.Join(dir, filename)
			loadedCfg, err := ReadMelangeConfig(fullPath)
			if err != nil {
				return p, fmt.Errorf("failed to read package config %s: %w", fullPath, err)
			}

			nolint, err := findNoLint(fullPath)
			if err != nil {
				return p, fmt.Errorf("failed to read package config %s: %w", fullPath, err)
			}

			p[loadedCfg.Package.Name] = &Packages{
				Config:   loadedCfg,
				Filename: filename,
				Dir:      dir,
				NoLint:   nolint,
			}
		}
		return p, nil
	}
	// get all packages in the provided git repo
	return ReadAllPackagesFromRepo(dir)
}

func findNoLint(filename string) ([]string, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#nolint:") {
			return strings.Split(strings.TrimPrefix(line, "#nolint:"), ","), nil
		}
	}
	return nil, nil
}

func ReadAllPackagesFromRepo(dir string) (map[string]*Packages, error) {
	p := make(map[string]*Packages)

	var fileList []string
	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if fi == nil {
			return fmt.Errorf("%s does not exist", dir)
		}
		if fi.IsDir() && path != dir {
			return filepath.SkipDir
		}
		if filepath.Ext(path) == ".yaml" {
			fileList = append(fileList, path)
		}
		return nil
	})
	if err != nil {
		return p, errors.Wrapf(err, "failed walking files in cloned directory %s", dir)
	}

	// guarantee a consistent sort order for test comparisons
	sort.Strings(fileList)

	for _, fi := range fileList {
		data, err := os.ReadFile(fi)
		if err != nil {
			return p, errors.Wrapf(err, "failed to read file %s", fi)
		}
		check := &ConfigCheck{}
		err = yaml.Unmarshal(data, check)
		if err != nil {
			// we need certain keys to unmarshal so we can identify this as a melange config, if there's no package name and version assume it is not a melange config
			continue
		}

		// skip if this file is not a melange config
		if !check.isMelangeConfig() {
			continue
		}

		packageConfig, err := ReadMelangeConfig(fi)
		if err != nil {
			return p, errors.Wrapf(err, "failed to read package config %s", fi)
		}
		relativeFilename, err := filepath.Rel(dir, fi)
		if err != nil {
			return p, errors.Wrapf(err, "failed to get relative path from dir %s and file %s package config %s", dir, fi, packageConfig.Package.Name)
		}

		nolint, err := findNoLint(fi)
		if err != nil {
			return p, fmt.Errorf("failed to read package config %s: %w", fi, err)
		}

		p[packageConfig.Package.Name] = &Packages{
			Config:   packageConfig,
			Filename: relativeFilename,
			Dir:      dir,
			NoLint:   nolint,
		}
	}
	fmt.Printf("found %[1]d packages\n", len(p))
	return p, nil
}

// ReadMelangeConfig reads a single melange config from the provided filename.
func ReadMelangeConfig(filename string) (config.Configuration, error) {
	packageConfig, err := config.ParseConfiguration(filename)
	if err != nil {
		return config.Configuration{}, err
	}
	return *packageConfig, err
}

func Bump(configFile, version, expectedCommit string) error {
	ctx, err := renovate.New(renovate.WithConfig(configFile))
	if err != nil {
		return err
	}

	bumpRenovator := bump.New(
		bump.WithTargetVersion(version),
		bump.WithExpectedCommit(expectedCommit),
	)

	return ctx.Renovate(context.TODO(), bumpRenovator)
}
