/*
 * Copyright 2018-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/dagger"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var buildpacks []string

func Package(root, version string, cached bool) (string, error) {
	var cmd *exec.Cmd

	bpPath := filepath.Join(root, "artifact")
	if cached {
		cmd = exec.Command(".bin/packager", "--archive", "--version", version, fmt.Sprintf("%s-cached", bpPath))
	} else {
		cmd = exec.Command(".bin/packager", "--archive", "--uncached", "--version", version, bpPath)
	}

	cmd.Env = append(os.Environ(), fmt.Sprintf("PACKAGE_DIR=%s", bpPath))
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if cached {
		return fmt.Sprintf("%s-cached.tgz", bpPath), err
	}

	return fmt.Sprintf("%s.tgz", bpPath), err
}

func TestIntegrationComposerApp(t *testing.T) {
	RegisterTestingT(t)

	var err error
	buildpacks, err = PreparePhpBps()
	Expect(err).ToNot(HaveOccurred())
	defer func() {
		for _, buildpack := range buildpacks {
			Expect(dagger.DeleteBuildpack(buildpack)).To(Succeed())
		}
	}()

	spec.Run(t, "Deploy A Composer App", testIntegrationComposerApp, spec.Report(report.Terminal{}), spec.Parallel())
}

func testIntegrationComposerApp(t *testing.T, when spec.G, it spec.S) {
	var (
		app *dagger.App
		err error
	)

	it.After(func() {
		if app != nil {
			Expect(app.Destroy()).To(Succeed())
		}
	})

	when("deploying a basic Composer app", func() {
		it("it deploys using defaults and installs a package using Composer", func() {
			app, err = PreparePhpApp("composer_app", buildpacks, false)
			Expect(err).ToNot(HaveOccurred())

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				containerID, imageName, volumeIDs, err := app.Info()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

				containerLogs, err := app.Logs()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("Container Logs:\n %s\n", containerLogs)
				t.FailNow()
			}

			// ensure composer library is available & functions
			logs, err := app.Logs()
			Expect(err).ToNot(HaveOccurred())
			Expect(logs).To(ContainSubstring("SUCCESS"))

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("OK"))
		})

		it("deploys using custom composer setting and installs a package using Composer", func() {
			app, err = PreparePhpApp("composer_app_custom", buildpacks, false)
			Expect(err).ToNot(HaveOccurred())

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				containerID, imageName, volumeIDs, err := app.Info()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

				containerLogs, err := app.Logs()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("Container Logs:\n %s\n", containerLogs)
				t.FailNow()
			}

			// ensure composer library is available & functions
			logs, err := app.Logs()
			Expect(err).ToNot(HaveOccurred())
			Expect(logs).To(ContainSubstring("SUCCESS"))

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("OK"))
		})

		it("deploys an app that has PHP extensions specified in composer.json", func() {
			ExpectedExtensions := []string{
				"zip",
				"gd",
				"fileinfo",
				"mysqli",
				"mbstring",
			}

			app, err = PreparePhpApp("composer_app_extensions", buildpacks, true)
			Expect(err).ToNot(HaveOccurred())

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				containerID, imageName, volumeIDs, err := app.Info()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

				containerLogs, err := app.Logs()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("Container Logs:\n %s\n", containerLogs)
				t.FailNow()
			}

			// ensure composer library is available & functions
			logs, err := app.Logs()
			Expect(err).ToNot(HaveOccurred())
			Expect(logs).To(ContainSubstring("SUCCESS"))

			buildLogs := app.BuildLogs()
			Expect(buildLogs).To(ContainSubstring("Running `php /layers/paketo-buildpacks_php-composer/composer/composer.phar config -g github-oauth.github.com "))

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("OK"))

			// ensure C extensions are loaded at runtime & during post-install scripts
			Expect(logs).ToNot(ContainSubstring("Unable to load dynamic library"))

			body, _, err = app.HTTPGet("/extensions.php")
			Expect(err).ToNot(HaveOccurred())
			for _, extension := range ExpectedExtensions {
				Expect(body).To(ContainSubstring(extension))
				Expect(app.BuildLogs()).To(ContainSubstring(fmt.Sprintf("PostInstall [%s]", extension)))
			}
		})

		it("deploys an app that installs global scripts using Composer and runs them as post scripts", func() {
			app, err = PreparePhpApp("composer_app_global", buildpacks, true)
			Expect(err).ToNot(HaveOccurred())

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				containerID, imageName, volumeIDs, err := app.Info()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("ContainerID: %s\nImage Name: %s\nAll leftover cached volumes: %v\n", containerID, imageName, volumeIDs)

				containerLogs, err := app.Logs()
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("Container Logs:\n %s\n", containerLogs)
				t.FailNow()
			}

			buildLogs := app.BuildLogs()
			Expect(buildLogs).To(ContainSubstring("Running `php /layers/paketo-buildpacks_php-composer/composer/composer.phar global require --no-progress friendsofphp/php-cs-fixer fxp/composer-asset-plugin:~1.3` from directory '/workspace'"))

			Expect(buildLogs).To(ContainSubstring("php-cs-fixer -h"))
			Expect(buildLogs).To(ContainSubstring("php /layers/paketo-buildpacks_php-composer/php-composer-packages/global/vendor/bin/php-cs-fixer list"))

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("OK"))
		})

		when("the app is pushed twice", func() {
			it("does not reinstall composer packages", func() {
				appName := "composer_app_extensions"
				debug := false
				app, err := PreparePhpApp(appName, buildpacks, debug)
				Expect(err).ToNot(HaveOccurred())

				Expect(app.BuildLogs()).To(MatchRegexp("Package operations: \\d+ install"))

				Expect(app.Start()).To(Succeed())

				// ensure composer library is available & functions
				logs, err := app.Logs()
				Expect(err).ToNot(HaveOccurred())
				Expect(logs).To(ContainSubstring("SUCCESS"))

				body, _, err := app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("OK"))

				// Second Run through
				app, err = dagger.PackBuildNamedImageWithEnv(app.ImageName, filepath.Join("testdata", appName), MakeBuildEnv(debug), buildpacks...)
				Expect(err).ToNot(HaveOccurred())

				Expect(app.BuildLogs()).To(MatchRegexp("PHP Composer \\S+: Reusing cached layer"))
				Expect(app.BuildLogs()).NotTo(MatchRegexp("PHP Composer \\S+: Contributing to layer"))

				Expect(app.Start()).To(Succeed())

				// ensure composer library is available & functions
				logs, err = app.Logs()
				Expect(err).ToNot(HaveOccurred())
				Expect(logs).To(ContainSubstring("SUCCESS"))

				body, _, err = app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("OK"))
			})

			it("does install composer packages", func() {
				appName := "composer_app_with_vendor"
				debug := false
				app, err := PreparePhpApp(appName, buildpacks, debug)
				Expect(err).ToNot(HaveOccurred())

				Expect(app.BuildLogs()).To(ContainSubstring("Nothing to install or update"))

				Expect(app.Start()).To(Succeed())

				// ensure composer library is available & functions
				logs, err := app.Logs()
				Expect(err).ToNot(HaveOccurred())
				Expect(logs).To(ContainSubstring("SUCCESS"))

				body, _, err := app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("OK"))

				// Second Run through
				app, err = dagger.PackBuildNamedImageWithEnv(app.ImageName, filepath.Join("testdata", appName), MakeBuildEnv(debug), buildpacks...)
				Expect(err).ToNot(HaveOccurred())

				Expect(app.BuildLogs()).To(MatchRegexp("PHP Composer \\S+: Reusing cached layer"))
				Expect(app.BuildLogs()).NotTo(MatchRegexp("PHP Composer \\S+: Contributing to layer"))

				Expect(app.Start()).To(Succeed())

				// ensure composer library is available & functions
				logs, err = app.Logs()
				Expect(err).ToNot(HaveOccurred())
				Expect(logs).To(ContainSubstring("SUCCESS"))

				body, _, err = app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("OK"))
			})

		})

		when("the app already has a vendor directory", func() {
			it("reuses the vendor'd dependencies", func() {
				appName := "composer_app_with_vendor"
				debug := false
				app, err := PreparePhpApp(appName, buildpacks, debug)
				Expect(err).ToNot(HaveOccurred())
				Expect(app.Start()).To(Succeed())

				Expect(app.BuildLogs()).ToNot(ContainSubstring("- Installing psr/log (1.1.1): Downloading (100%)"))
				Expect(app.BuildLogs()).ToNot(ContainSubstring("- Installing monolog/monolog (1.25.1): Downloading (100%)"))

				// ensure composer library is available & functions
				logs, err := app.Logs()
				Expect(err).ToNot(HaveOccurred())
				Expect(logs).To(ContainSubstring("SUCCESS"))

				body, _, err := app.HTTPGet("/")
				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(ContainSubstring("OK"))
			})
		})
	})
}
