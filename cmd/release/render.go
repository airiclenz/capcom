package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// The package-channel files publish writes, rendered whole from the templates below at each
// release. The templates are the source of truth: the tap's formula, the bucket's manifest and
// the winget manifests say they are generated here, so a fix to any of them lands in this
// repository, never in theirs. Every template base name is distinct, since templates are looked
// up by base name whichever directory they sit in.
const (
	formulaTemplate             = "capcom.rb.tmpl"
	scoopTemplate               = "scoop.json.tmpl"
	wingetVersionTemplate       = "winget.version.yaml.tmpl"
	wingetInstallerTemplate     = "winget.installer.yaml.tmpl"
	wingetDefaultLocaleTemplate = "winget.defaultLocale.yaml.tmpl"
	packageTemplatesPattern     = "templates/*.tmpl"
	wingetTemplatesPattern      = "templates/winget/*.tmpl"
)

// The base URLs of a release on GitHub: every asset downloads from releaseDownloadBase and the
// release page sits at releasePageBase, each followed by the tag.
const (
	releaseDownloadBase = "https://github.com/" + capcomRepo + "/releases/download/"
	releasePageBase     = "https://github.com/" + capcomRepo + "/releases/tag/"
)

//go:embed templates/*.tmpl templates/winget/*.tmpl
var templateFiles embed.FS

// packageTemplates holds every package-channel template, parsed once. A template that fails to
// parse is a defect of this program, caught by every render test, so it panics at start-up.
var packageTemplates = template.Must(template.ParseFS(templateFiles, packageTemplatesPattern, wingetTemplatesPattern))

// renderedFile is one rendered package-channel file and the name it is written under.
type renderedFile struct {
	name    string
	content string
}

// wingetManifest is one manifest of a winget submission: the file name komac reads it under and
// the template it renders from.
type wingetManifest struct {
	name     string
	template string
}

// wingetManifests are the three manifests a winget submission carries, in komac's file naming:
// the version manifest, the installer manifest and the en-US default-locale manifest.
var wingetManifests = []wingetManifest{
	{name: wingetPackageID + ".yaml", template: wingetVersionTemplate},
	{name: wingetPackageID + ".installer.yaml", template: wingetInstallerTemplate},
	{name: wingetPackageID + ".locale.en-US.yaml", template: wingetDefaultLocaleTemplate},
}

// checksumPattern is the one hash shape SHA256SUMS may list for an archive: 64 lower-case hex
// digits, after lower-casing.
var checksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// The two ways a SHA256SUMS body can fail to name an archive's hash.
var (
	errNoChecksum        = errors.New("SHA256SUMS lists no hash")
	errMalformedChecksum = errors.New("SHA256SUMS lists a malformed hash")
)

// packageArchive is one release archive as a package-channel file names it.
type packageArchive struct {
	// URL is the archive's download URL on the release.
	URL string
	// SHA256 is the archive's hash, as SHA256SUMS lists it.
	SHA256 string
	// Stem is the archive's name without its extension, and its one top-level directory.
	Stem string
}

// UpperSHA256 is the archive's hash in upper case, the form komac writes into winget manifests.
func (a packageArchive) UpperSHA256() string {
	return strings.ToUpper(a.SHA256)
}

// packageData is what a package-channel template renders from.
type packageData struct {
	// Version is the bare release version, X.Y.Z.
	Version string
	tag     string
	sums    string
}

// ReleaseURL is the release's page on GitHub, which the winget locale manifest links as its
// release notes.
func (d packageData) ReleaseURL() string {
	return releasePageBase + d.tag
}

// WingetPackageID is the package identifier capcom is published under in winget-pkgs.
func (d packageData) WingetPackageID() string {
	return wingetPackageID
}

// Archive resolves the archive a template names by its target. Templates call it for exactly the
// archives they install, so a SHA256SUMS that omits an archive the template does not use is no
// error.
func (d packageData) Archive(goos, goarch string) (packageArchive, error) {
	target := distTarget{goos: goos, goarch: goarch}
	if !isDistTarget(target) {
		return packageArchive{}, fmt.Errorf("%s/%s is not a release target", goos, goarch)
	}

	name := target.archiveName(d.Version)
	hash, err := checksumFor(d.sums, name)
	if err != nil {
		return packageArchive{}, err
	}
	return packageArchive{
		URL:    releaseDownloadBase + d.tag + "/" + name,
		SHA256: hash,
		Stem:   target.stem(d.Version),
	}, nil
}

// isDistTarget reports whether target is one of the platforms a release ships.
func isDistTarget(target distTarget) bool {
	for _, known := range distTargets {
		if known == target {
			return true
		}
	}
	return false
}

// checksumFor returns the hash the SHA256SUMS body sums lists for the archive name, lower-cased.
// Both the text (`hash  name`) and binary (`hash *name`) line forms are read; the first line
// naming the archive wins, as `sha256sum --check` would read it.
func checksumFor(sums, name string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[1] != name && fields[1] != "*"+name) {
			continue
		}
		hash := strings.ToLower(fields[0])
		if !checksumPattern.MatchString(hash) {
			return "", fmt.Errorf("%w for %s: %q", errMalformedChecksum, name, fields[0])
		}
		return hash, nil
	}
	return "", fmt.Errorf("%w for %s", errNoChecksum, name)
}

// renderWingetManifests renders the three winget manifests for version (vX.Y.Z or a bare X.Y.Z),
// reading the two Windows archive hashes from the release's SHA256SUMS body, and returns them in
// wingetManifests' order under the file names a `komac submit` directory holds them by.
func renderWingetManifests(version, sums string) ([]renderedFile, error) {
	rendered := make([]renderedFile, 0, len(wingetManifests))
	for _, manifest := range wingetManifests {
		content, err := renderPackageFile(manifest.template, version, sums)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, renderedFile{name: manifest.name, content: content})
	}
	return rendered, nil
}

// renderPackageFile executes the named template for version against the SHA256SUMS body sums.
func renderPackageFile(name, version, sums string) (string, error) {
	parsed, err := parseReleaseVersion(version)
	if err != nil {
		return "", err
	}

	tag := parsed.String()
	data := packageData{Version: strings.TrimPrefix(tag, "v"), tag: tag, sums: sums}
	var rendered bytes.Buffer
	if err := packageTemplates.ExecuteTemplate(&rendered, name, data); err != nil {
		return "", fmt.Errorf("render %s for %s: %w", name, tag, err)
	}
	return rendered.String(), nil
}
