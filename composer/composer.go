package composer

import (
	"fmt"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/php-composer-cnb/runner"
	"github.com/cloudfoundry/php-web-cnb/phpweb"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	Dependency      = "php-composer"
	CacheDependency = "php-composer-cache"
	ComposerLock    = "composer.lock"
	ComposerJSON    = "composer.json"
	ComposerPHAR    = "composer.phar"
	GithubOAUTHKey  = "github-oauth.github.com"
)

type Composer struct {
	Runner     runner.Runner
	workingDir string
	pharPath   string
}

func NewComposer(composerJsonPath, composerPharPath string) Composer {
	return Composer{
		Runner:     runner.ComposerRunner{},
		workingDir: composerJsonPath,
		pharPath:   filepath.Join(composerPharPath, ComposerPHAR),
	}
}

func (c Composer) Install(args ...string) error {
	args = append([]string{c.pharPath, "install", "--no-progress"}, args...)
	return c.Runner.Run("php", c.workingDir, args...)
}

func (c Composer) Version() error {
	return c.Runner.Run("php", c.workingDir, c.pharPath, "-V")
}

func (c Composer) Global(args ...string) error {
	args = append([]string{c.pharPath, "global", "require", "--no-progress"}, args...)
	return c.Runner.Run("php", c.workingDir, args...)
}

func (c Composer) Config(key, value string, global bool) error {
	args := []string{c.pharPath, "config"}
	if global {
		args = append(args, "-g")
	}
	args = append(args, key, fmt.Sprintf(`"%s"`, value))
	return c.Runner.Run("php", c.workingDir, args...)
}

// FindComposer locates the composer JSON and composer lock files
func FindComposer(appRoot string, composerJSONPath string) (string, error) {
	composerJSON := filepath.Join(appRoot, ComposerJSON)

	if exists, err := helper.FileExists(composerJSON); err != nil {
		return "", fmt.Errorf("error checking filepath: %s", composerJSON)
	} else if exists {
		return composerJSON, nil
	}

	phpBuildpackYAML, err := phpweb.LoadBuildpackYAML(appRoot)
	if err != nil {
		return "", err
	}

	composerJSON = filepath.Join(appRoot, phpBuildpackYAML.Config.WebDirectory, composerJSONPath, ComposerJSON)
	if exists, err := helper.FileExists(composerJSON); err != nil {
		return "", fmt.Errorf("error checking filepath: %s", composerJSON)
	} else if exists {
		return composerJSON, nil
	}

	return "", fmt.Errorf(`no "%s" found at: %s`, ComposerJSON, composerJSON)
}

type ComposerConfig struct {
	Version          string   `yaml:"version"`
	InstallOptions   []string `yaml:"install_options"`
	VendorDirectory  string   `yaml:"vendor_directory"`
	JsonPath         string   `yaml:"json_path"`
	GitHubOAUTHToken string   `yaml:"github_oauth_token"`
}

type BuildpackYAML struct {
	Composer ComposerConfig `yaml:"composer"`
}

// LoadComposerBuildpackYAML loads the buildpack YAML from disk
func LoadComposerBuildpackYAML(appRoot string) (BuildpackYAML, error) {
	buildpackYAML, configFile := BuildpackYAML{}, filepath.Join(appRoot, "buildpack.yml")
	if exists, err := helper.FileExists(configFile); err != nil {
		return BuildpackYAML{}, err
	} else if exists {
		file, err := os.Open(configFile)
		if err != nil {
			return BuildpackYAML{}, err
		}
		defer file.Close()

		contents, err := ioutil.ReadAll(file)
		if err != nil {
			return BuildpackYAML{}, err
		}

		err = yaml.Unmarshal(contents, &buildpackYAML)
		if err != nil {
			return BuildpackYAML{}, err
		}
	}
	return buildpackYAML, nil
}
