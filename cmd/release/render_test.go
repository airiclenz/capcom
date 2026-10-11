package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// goldenVersion is the release the golden package files under testdata are rendered for.
const goldenVersion = "0.0.0"

// The fixture hashes the formula tests read, one per macOS and Linux archive.
const (
	formulaDarwinArm64Hash = "1111111111111111111111111111111111111111111111111111111111111111"
	formulaDarwinAmd64Hash = "2222222222222222222222222222222222222222222222222222222222222222"
	formulaLinuxArm64Hash  = "3333333333333333333333333333333333333333333333333333333333333333"
	formulaLinuxAmd64Hash  = "4444444444444444444444444444444444444444444444444444444444444444"
)

// The fixture hashes the Scoop and winget tests read, one per Windows archive plus one for an
// archive neither installs.
const (
	scoopAmd64Hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	scoopArm64Hash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	scoopLinuxHash = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

// renderedScoopArchitecture is one entry of a rendered Scoop manifest's `architecture` block.
type renderedScoopArchitecture struct {
	URL        string `json:"url"`
	Hash       string `json:"hash"`
	ExtractDir string `json:"extract_dir"`
}

// renderedScoopManifest is the part of a rendered bucket/capcom.json the tests assert.
type renderedScoopManifest struct {
	GeneratedNote string                               `json:"##"`
	Version       string                               `json:"version"`
	Description   string                               `json:"description"`
	Homepage      string                               `json:"homepage"`
	License       string                               `json:"license"`
	Architecture  map[string]renderedScoopArchitecture `json:"architecture"`
	Bin           string                               `json:"bin"`
	Checkver      string                               `json:"checkver"`
	Autoupdate    struct {
		Architecture map[string]json.RawMessage `json:"architecture"`
	} `json:"autoupdate"`
}

// goldenSums is the SHA256SUMS body the golden files are rendered from: the four formula
// archives and the two Windows archives of goldenVersion.
func goldenSums() string {
	return formulaLinuxAmd64Hash + "  capcom_0.0.0_linux_amd64.tar.gz\n" +
		formulaLinuxArm64Hash + "  capcom_0.0.0_linux_arm64.tar.gz\n" +
		formulaDarwinAmd64Hash + "  capcom_0.0.0_darwin_amd64.tar.gz\n" +
		formulaDarwinArm64Hash + "  capcom_0.0.0_darwin_arm64.tar.gz\n" +
		scoopAmd64Hash + "  capcom_0.0.0_windows_amd64.zip\n" +
		scoopArm64Hash + "  capcom_0.0.0_windows_arm64.zip\n"
}

// TestRenderFormulaMatchesTheGoldenFormula renders goldenVersion, tagged and bare, and expects
// testdata/capcom-0.0.0.rb byte for byte.
func TestRenderFormulaMatchesTheGoldenFormula(t *testing.T) {
	t.Parallel()
	want := readTestFile(t, filepath.Join("testdata", "capcom-"+goldenVersion+".rb"))

	for _, version := range []string{"v" + goldenVersion, goldenVersion} {
		got, err := renderFormula(version, goldenSums())
		if err != nil {
			t.Fatalf("renderFormula(%q) = %v", version, err)
		}
		if got != want {
			t.Errorf("renderFormula(%q) differs from testdata/capcom-%s.rb:\n%s", version, goldenVersion, got)
		}
	}
}

// TestRenderScoopManifest asserts the rendered manifest field by field: version, homepage,
// description, license, bin, checkver, both Windows architectures and their autoupdate entries.
func TestRenderScoopManifest(t *testing.T) {
	t.Parallel()

	const (
		bare        = "9.8.7"
		releaseBase = "https://github.com/airiclenz/capcom/releases/download/v" + bare + "/"
	)
	sums := scoopLinuxHash + "  capcom_" + bare + "_linux_amd64.tar.gz\n" +
		scoopAmd64Hash + "  capcom_" + bare + "_windows_amd64.zip\n" +
		scoopArm64Hash + "  capcom_" + bare + "_windows_arm64.zip\n"

	rendered, err := renderScoopManifest("v"+bare, sums)
	if err != nil {
		t.Fatalf("renderScoopManifest = %v", err)
	}

	var manifest renderedScoopManifest
	if err := json.Unmarshal([]byte(rendered), &manifest); err != nil {
		t.Fatalf("the rendered manifest is not JSON: %v\n%s", err, rendered)
	}
	if !strings.Contains(manifest.GeneratedNote, "cmd/release") || !strings.Contains(manifest.GeneratedNote, "airiclenz/capcom") {
		t.Errorf(`"##" = %q; want the generated-by-cmd/release note`, manifest.GeneratedNote)
	}
	if manifest.Version != bare {
		t.Errorf("version = %q; want %q", manifest.Version, bare)
	}
	if manifest.Homepage != "https://github.com/airiclenz/capcom" {
		t.Errorf("homepage = %q; want the release repository's GitHub page", manifest.Homepage)
	}
	if manifest.Description != "Git-native issue tracker and knowledge store for coding agents" || manifest.License != "MIT" {
		t.Errorf("description %q / license %q; want capcom's description and MIT", manifest.Description, manifest.License)
	}
	if manifest.Bin != "capcom.exe" || manifest.Checkver != "github" {
		t.Errorf("bin = %q, checkver = %q; want capcom.exe and github", manifest.Bin, manifest.Checkver)
	}
	wants := map[string]renderedScoopArchitecture{
		"64bit": {
			URL:        releaseBase + "capcom_" + bare + "_windows_amd64.zip",
			Hash:       scoopAmd64Hash,
			ExtractDir: "capcom_" + bare + "_windows_amd64",
		},
		"arm64": {
			URL:        releaseBase + "capcom_" + bare + "_windows_arm64.zip",
			Hash:       scoopArm64Hash,
			ExtractDir: "capcom_" + bare + "_windows_arm64",
		},
	}
	for key, want := range wants {
		if got := manifest.Architecture[key]; got != want {
			t.Errorf("architecture.%s = %+v; want %+v", key, got, want)
		}
		if _, ok := manifest.Autoupdate.Architecture[key]; !ok {
			t.Errorf("autoupdate.architecture has no %s entry", key)
		}
	}
	if !strings.Contains(rendered, `"url": "$baseurl/SHA256SUMS"`) {
		t.Errorf("autoupdate does not hash from $baseurl/SHA256SUMS:\n%s", rendered)
	}
}

// TestRenderWingetManifestsMatchTheGoldenManifests renders goldenVersion's three winget
// manifests, tagged and bare, and expects each file under testdata/winget byte for byte, under
// komac's file names and as valid YAML.
func TestRenderWingetManifestsMatchTheGoldenManifests(t *testing.T) {
	t.Parallel()
	wantNames := []string{"AiricLenz.Capcom.yaml", "AiricLenz.Capcom.installer.yaml", "AiricLenz.Capcom.locale.en-US.yaml"}

	for _, version := range []string{"v" + goldenVersion, goldenVersion} {
		manifests, err := renderWingetManifests(version, goldenSums())
		if err != nil {
			t.Fatalf("renderWingetManifests(%q) = %v", version, err)
		}
		if len(manifests) != len(wantNames) {
			t.Fatalf("renderWingetManifests(%q) rendered %d manifests; want %d", version, len(manifests), len(wantNames))
		}
		for i, manifest := range manifests {
			if manifest.name != wantNames[i] {
				t.Errorf("manifest %d is named %q; want %q", i, manifest.name, wantNames[i])
			}
			want := readTestFile(t, filepath.Join("testdata", "winget", wantNames[i]))
			if manifest.content != want {
				t.Errorf("renderWingetManifests(%q): %s differs from testdata/winget:\n%s", version, wantNames[i], manifest.content)
			}
			var parsed map[string]any
			if err := yaml.Unmarshal([]byte(manifest.content), &parsed); err != nil {
				t.Errorf("%s is not valid YAML: %v", manifest.name, err)
			}
		}
	}
}

// TestChecksumForReadsBothLineForms reads a hash from the text and the binary SHA256SUMS line
// forms, and lower-cases an upper-case one.
func TestChecksumForReadsBothLineForms(t *testing.T) {
	t.Parallel()

	const name = "capcom_1.2.3_windows_amd64.zip"
	tests := map[string]string{
		"text form":          scoopAmd64Hash + "  " + name + "\n",
		"binary form":        scoopAmd64Hash + " *" + name + "\n",
		"upper case":         strings.ToUpper(scoopAmd64Hash) + "  " + name + "\n",
		"crlf line endings":  scoopLinuxHash + "  other.zip\r\n" + scoopAmd64Hash + "  " + name + "\r\n",
		"first match wins":   scoopAmd64Hash + "  " + name + "\n" + scoopArm64Hash + "  " + name + "\n",
		"suffix is no match": scoopArm64Hash + "  x" + name + "\n" + scoopAmd64Hash + "  " + name + "\n",
	}
	for label, sums := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			got, err := checksumFor(sums, name)
			if err != nil {
				t.Fatalf("checksumFor = %v", err)
			}
			if got != scoopAmd64Hash {
				t.Errorf("checksumFor = %q; want %q", got, scoopAmd64Hash)
			}
		})
	}
}

// TestRenderRefusesABadOrMissingHash refuses to render any package file when SHA256SUMS omits an
// archive the file installs or lists a hash that is not 64 hex digits.
func TestRenderRefusesABadOrMissingHash(t *testing.T) {
	t.Parallel()

	const (
		bare         = "1.2.3"
		darwinArm64  = "capcom_1.2.3_darwin_arm64.tar.gz"
		windowsAmd64 = "capcom_1.2.3_windows_amd64.zip"
		windowsArm64 = "capcom_1.2.3_windows_arm64.zip"
	)
	// otherFormulaSums lists the three formula archives every formula case leaves intact.
	otherFormulaSums := formulaDarwinAmd64Hash + "  capcom_1.2.3_darwin_amd64.tar.gz\n" +
		formulaLinuxArm64Hash + "  capcom_1.2.3_linux_arm64.tar.gz\n" +
		formulaLinuxAmd64Hash + "  capcom_1.2.3_linux_amd64.tar.gz\n"
	amd64Sums := scoopAmd64Hash + "  " + windowsAmd64 + "\n"
	renderWinget := func(version, sums string) (string, error) {
		_, err := renderWingetManifests(version, sums)
		return "", err
	}
	render := map[string]func(string, string) (string, error){
		"formula": renderFormula,
		"scoop":   renderScoopManifest,
		"winget":  renderWinget,
	}

	tests := map[string]struct {
		file    string
		sums    string
		wantErr error
		archive string
	}{
		"formula: archive missing": {file: "formula", sums: otherFormulaSums, wantErr: errNoChecksum, archive: darwinArm64},
		"formula: short hash":      {file: "formula", sums: formulaDarwinArm64Hash[1:] + "  " + darwinArm64 + "\n" + otherFormulaSums, wantErr: errMalformedChecksum, archive: darwinArm64},
		"formula: not hex":         {file: "formula", sums: "z" + formulaDarwinArm64Hash[1:] + "  " + darwinArm64 + "\n" + otherFormulaSums, wantErr: errMalformedChecksum, archive: darwinArm64},
		"scoop: archive missing":   {file: "scoop", sums: amd64Sums, wantErr: errNoChecksum, archive: windowsArm64},
		"scoop: long hash":         {file: "scoop", sums: amd64Sums + scoopArm64Hash + "b  " + windowsArm64 + "\n", wantErr: errMalformedChecksum, archive: windowsArm64},
		"scoop: empty sums":        {file: "scoop", sums: "", wantErr: errNoChecksum, archive: windowsAmd64},
		"winget: archive missing":  {file: "winget", sums: amd64Sums, wantErr: errNoChecksum, archive: windowsArm64},
		"winget: empty sums":       {file: "winget", sums: "", wantErr: errNoChecksum, archive: windowsAmd64},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			got, err := render[tc.file]("v"+bare, tc.sums)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("render = %q, %v; want %v", got, err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.archive) {
				t.Errorf("error %q does not name %s", err, tc.archive)
			}
		})
	}
}

// TestRenderRefusesABadVersion refuses a version that is not vX.Y.Z or a bare X.Y.Z before it
// reads a single hash.
func TestRenderRefusesABadVersion(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"", "v1.2", "1.2.3-rc1", "v01.2.3", "latest"} {
		if _, err := renderFormula(version, ""); err == nil || !strings.Contains(err.Error(), "not a release version") {
			t.Errorf("renderFormula(%q) = %v; want a not-a-release-version error", version, err)
		}
		if _, err := renderScoopManifest(version, ""); err == nil || !strings.Contains(err.Error(), "not a release version") {
			t.Errorf("renderScoopManifest(%q) = %v; want a not-a-release-version error", version, err)
		}
		if _, err := renderWingetManifests(version, ""); err == nil || !strings.Contains(err.Error(), "not a release version") {
			t.Errorf("renderWingetManifests(%q) = %v; want a not-a-release-version error", version, err)
		}
	}
}

// TestPackageDataRefusesAnUnknownTarget refuses an archive for a platform no release ships, so a
// template typo fails the render rather than pinning a URL that does not exist.
func TestPackageDataRefusesAnUnknownTarget(t *testing.T) {
	t.Parallel()

	data := packageData{Version: "1.2.3", tag: "v1.2.3", sums: scoopAmd64Hash + "  capcom_1.2.3_freebsd_amd64.tar.gz\n"}
	if _, err := data.Archive("freebsd", "amd64"); err == nil || !strings.Contains(err.Error(), "not a release target") {
		t.Errorf("Archive(freebsd, amd64) = %v; want a not-a-release-target error", err)
	}
}

// TestPackageDataReleaseURL pins the release page the winget locale manifest links: the tagged
// release on capcom's GitHub repository.
func TestPackageDataReleaseURL(t *testing.T) {
	t.Parallel()

	data := packageData{Version: "1.2.3", tag: "v1.2.3"}
	if got, want := data.ReleaseURL(), "https://github.com/airiclenz/capcom/releases/tag/v1.2.3"; got != want {
		t.Errorf("ReleaseURL() = %q; want %q", got, want)
	}
}
