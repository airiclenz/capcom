package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestExecChannelReposRunsEachMethodsArgv pins the gh argv behind every execChannelRepos method,
// run in its directory, and what each makes of the answer: a yes, and the "no" a 404 — a missing
// repository or path, or an empty repository — reads as.
func TestExecChannelReposRunsEachMethodsArgv(t *testing.T) {
	t.Parallel()

	tapRepoCall := recordedCall{name: "gh", args: []string{"api", "repos/airiclenz/homebrew-tap", "--jq", ".full_name"}}
	formulaCall := recordedCall{name: "gh", args: []string{"api", "repos/airiclenz/homebrew-tap/contents/Formula/capcom.rb", "--jq", ".content"}}
	listingCall := recordedCall{name: "gh", args: []string{"api", "repos/microsoft/winget-pkgs/contents/manifests/a/AiricLenz/Capcom"}}
	pullsCall := recordedCall{name: "gh", args: []string{"pr", "list", "--repo", "microsoft/winget-pkgs", "--state", "open",
		"--search", "AiricLenz.Capcom 0.24.11 in:title", "--json", "number,title,url"}}
	exists := func(ctx context.Context, channels execChannelRepos) (any, error) {
		return channels.Exists(ctx, tapRepo)
	}
	readFile := func(ctx context.Context, channels execChannelRepos) (any, error) {
		content, isFound, err := channels.ReadFile(ctx, tapRepo, formulaPath)
		return []any{content, isFound}, err
	}
	listDirs := func(ctx context.Context, channels execChannelRepos) (any, error) {
		dirs, isFound, err := channels.ListDirs(ctx, wingetRepo, wingetManifestDir)
		return []any{dirs, isFound}, err
	}
	openPulls := func(ctx context.Context, channels execChannelRepos) (any, error) {
		return channels.OpenPulls(ctx, wingetRepo, wingetPackageID+" 0.24.11 in:title")
	}

	cases := map[string]struct {
		call     func(ctx context.Context, channels execChannelRepos) (any, error)
		output   string
		err      error
		want     any
		wantCall recordedCall
	}{
		"Exists yes":                      {call: exists, output: tapRepo + "\n", want: true, wantCall: tapRepoCall},
		"Exists no":                       {call: exists, err: errors.New(ghNotFound), want: false, wantCall: tapRepoCall},
		"ReadFile yes":                    {call: readFile, output: contentsAPIBase64(testFormula), want: []any{testFormula, true}, wantCall: formulaCall},
		"ReadFile no":                     {call: readFile, err: errors.New(ghNotFound), want: []any{"", false}, wantCall: formulaCall},
		"ReadFile in an empty repository": {call: readFile, err: errors.New(ghEmptyRepository), want: []any{"", false}, wantCall: formulaCall},
		"ListDirs yes keeps only the directories": {call: listDirs,
			output:   `[{"name":"0.24.10","type":"dir"},{"name":"README.md","type":"file"},{"name":"0.24.11","type":"dir"}]` + "\n",
			want:     []any{[]string{"0.24.10", "0.24.11"}, true},
			wantCall: listingCall},
		"ListDirs no":                     {call: listDirs, err: errors.New(ghNotFound), want: []any{[]string(nil), false}, wantCall: listingCall},
		"ListDirs in an empty repository": {call: listDirs, err: errors.New(ghEmptyRepository), want: []any{[]string(nil), false}, wantCall: listingCall},
		"OpenPulls with a pull request": {call: openPulls, output: wingetPull(312345, "New version", "0.24.11"),
			want: []pullRequest{{number: 312345, title: "New version: AiricLenz.Capcom version 0.24.11",
				url: "https://github.com/microsoft/winget-pkgs/pull/312345"}},
			wantCall: pullsCall},
		"OpenPulls with none": {call: openPulls, output: "[]\n", want: []pullRequest{}, wantCall: pullsCall},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := answering(tc.output, tc.err)
			channels := execChannelRepos{commands: commands, dir: "/work"}

			got, err := tc.call(t.Context(), channels)

			if err != nil {
				t.Fatalf("%s failed: %v", name, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s = %#v, want %#v", name, got, tc.want)
			}
			wantCall := tc.wantCall
			wantCall.dir = "/work"
			if len(commands.calls) != 1 || !reflect.DeepEqual(commands.calls[0], wantCall) {
				t.Errorf("calls = %+v, want exactly %+v", commands.calls, wantCall)
			}
		})
	}
}

// TestExecChannelReposTellsANoFromABrokenCommand pins execChannelRepos' classification: a gh
// failure other than a 404 is the runner's error, and an answer the adapter cannot parse is an
// error wrapping errUnparsableAnswer that reads as the parser's own message.
func TestExecChannelReposTellsANoFromABrokenCommand(t *testing.T) {
	t.Parallel()
	brokenCall := errors.New("exit status 1: gh: Server Error (HTTP 500)")

	cases := map[string]struct {
		call           func(ctx context.Context, channels execChannelRepos) error
		output         string
		err            error
		wantError      string
		wantUnparsable bool
	}{
		"Exists on a broken call": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, err := channels.Exists(ctx, bucketRepo)
				return err
			},
			err:       brokenCall,
			wantError: brokenCall.Error(),
		},
		"ReadFile on a broken call": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, _, err := channels.ReadFile(ctx, bucketRepo, bucketPath)
				return err
			},
			err:       brokenCall,
			wantError: brokenCall.Error(),
		},
		"ReadFile on content that is not base64": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, _, err := channels.ReadFile(ctx, bucketRepo, bucketPath)
				return err
			},
			output:         "not base64!\n",
			wantError:      "decode bucket/capcom.json: illegal base64 data",
			wantUnparsable: true,
		},
		"ListDirs on a broken call": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, _, err := channels.ListDirs(ctx, wingetRepo, wingetManifestDir)
				return err
			},
			err:       brokenCall,
			wantError: brokenCall.Error(),
		},
		"ListDirs on a listing that is not JSON": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, _, err := channels.ListDirs(ctx, wingetRepo, wingetManifestDir)
				return err
			},
			output:         "not json",
			wantError:      "invalid character 'o' in literal null (expecting 'u')",
			wantUnparsable: true,
		},
		"OpenPulls on a broken call": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, err := channels.OpenPulls(ctx, wingetRepo, wingetPackageID)
				return err
			},
			err:       brokenCall,
			wantError: brokenCall.Error(),
		},
		"OpenPulls on output that is not JSON": {
			call: func(ctx context.Context, channels execChannelRepos) error {
				_, err := channels.OpenPulls(ctx, wingetRepo, wingetPackageID)
				return err
			},
			output:         "not json",
			wantError:      "invalid character 'o' in literal null (expecting 'u')",
			wantUnparsable: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			channels := execChannelRepos{commands: answering(tc.output, tc.err), dir: "."}

			err := tc.call(t.Context(), channels)

			if err == nil || !strings.HasPrefix(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want one starting %q", err, tc.wantError)
			}
			if errors.Is(err, errUnparsableAnswer) != tc.wantUnparsable {
				t.Errorf("errors.Is(%v, errUnparsableAnswer) = %t, want %t", err, !tc.wantUnparsable, tc.wantUnparsable)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("error %v does not wrap the runner's %v", err, tc.err)
			}
		})
	}
}
