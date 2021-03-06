package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	bplogger "github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/buildpackplan"
	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/paketo-buildpacks/php-composer/composer"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

func TestUnitDetect(t *testing.T) {
	spec.Run(t, "Detect", testDetect, spec.Report(report.Terminal{}))
}

func testDetect(t *testing.T, when spec.G, it spec.S) {
	var factory *test.DetectFactory

	it.Before(func() {
		RegisterTestingT(t)
		factory = test.NewDetectFactory(t)
	})

	when("there is a composer.json with a php version", func() {
		var compsoserPath string
		var phpVersion string
		it.Before(func() {
			phpVersion = ">=5.6"
			composerJSONString := `{"require": {"php": "` + phpVersion + `"}}`
			compsoserPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerJSON)
			test.WriteFile(t, compsoserPath, composerJSONString)
		})

		it("should parse the correct version", func() {
			version, _, err := findPHPVersion(compsoserPath, factory.Detect.Logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(phpVersion))
		})
	})

	when("there is a composer.json and a composer.lock with a php version", func() {
		var (
			compsoserPath    string
			composerLockPath string
			phpVersion       string
			phpLockVersion   string
		)

		it.Before(func() {
			phpVersion = ">=5.6"
			composerJSONString := `{"require": {"php": "` + phpVersion + `"}}`
			compsoserPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerJSON)
			test.WriteFile(t, compsoserPath, composerJSONString)

			phpLockVersion = ">=7.0"
			composerLockString := `{"platform": {"php": "` + phpLockVersion + `"}}`
			composerLockPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerLock)
			test.WriteFile(t, composerLockPath, composerLockString)
		})

		it("should parse the version from composer.lock", func() {
			version, _, err := findPHPVersion(compsoserPath, factory.Detect.Logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(phpLockVersion))
		})
	})

	when("there is a composer.json but not a composer.lock", func() {
		var (
			compsoserPath string
			phpVersion    string
		)

		it.Before(func() {
			phpVersion = ">=5.6"
			composerJSONString := `{"require": {"php": "` + phpVersion + `"}}`
			compsoserPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerJSON)
			test.WriteFile(t, compsoserPath, composerJSONString)
		})

		it("should parse the version from composer.json", func() {
			debug := &bytes.Buffer{}
			info := &bytes.Buffer{}

			log := logger.Logger{Logger: bplogger.NewLogger(debug, info)}

			version, _, err := findPHPVersion(compsoserPath, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(phpVersion))
			Expect(info.String()).To(Equal("WARNING: Include a 'composer.lock' file with your application! This will make sure the exact same version of dependencies are used when you deploy to CloudFoundry. It will also enable caching of your dependency layer.\n"))
		})
	})

	when("there is a composer.json and a composer.lock and neither have a php version", func() {
		var (
			composerPath     string
			composerLockPath string
		)

		it.Before(func() {
			composerJSONString := `{"require": {}}`
			composerPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerJSON)
			test.WriteFile(t, composerPath, composerJSONString)

			composerLockString := `{"platform": []}`
			composerLockPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerLock)
			test.WriteFile(t, composerLockPath, composerLockString)
		})

		it("should not fail, but just pick latest PHP", func() {
			version, _, err := findPHPVersion(composerPath, factory.Detect.Logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})
	})

	when("composer is being used", func() {
		const VERSION string = "1.2.3"
		var composerPath string

		it.Before(func() {
			composerJSONString := `{"require": {"php": "` + VERSION + `"}}`

			composerPath = filepath.Join(factory.Detect.Application.Root, composer.ComposerJSON)
			test.WriteFile(t, composerPath, composerJSONString)
			fakeVersion := "php.default.VERSION"
			factory.Detect.Buildpack.Metadata = map[string]interface{}{"default_version": fakeVersion}
		})

		when("there is no composer version specified in buildpack.yml", func() {
			it("should contribute to the build plan with the default composer version", func() {
				code, err := runDetect(factory.Detect)
				Expect(err).NotTo(HaveOccurred())
				Expect(code).To(Equal(detect.PassStatusCode))

				Expect(factory.Plans.Plan).To(Equal(buildplan.Plan{
					Requires: []buildplan.Required{
						{
							Name:    "php",
							Version: VERSION,
							Metadata: buildplan.Metadata{
								"build":                     true,
								buildpackplan.VersionSource: "composer.json",
							},
						},
						{
							Name: composer.Dependency,
						},
					},
					Provides: []buildplan.Provided{{Name: composer.Dependency}},
				}))
			})
		})

		when("there is a buildpack.yml", func() {
			it("should contribute to the build plan with the specified composer version", func() {
				test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, "buildpack.yml"), `{"composer": {"version": "1.2.3"}}`)

				code, err := runDetect(factory.Detect)
				Expect(err).NotTo(HaveOccurred())
				Expect(code).To(Equal(detect.PassStatusCode))

				Expect(factory.Plans.Plan).To(Equal(buildplan.Plan{
					Requires: []buildplan.Required{
						{
							Name:    "php",
							Version: VERSION,
							Metadata: buildplan.Metadata{
								"build":                     true,
								buildpackplan.VersionSource: "composer.json",
							},
						},
						{
							Name:    composer.Dependency,
							Version: "1.2.3",
						},
					},
					Provides: []buildplan.Provided{{Name: composer.Dependency}},
				}))
			})
		})
	})

	when("there is no composer.json", func() {
		it("should NOT contribute to the build plan", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).To(HaveOccurred())
			Expect(code).To(Equal(detect.FailStatusCode))
		})
	})
}
