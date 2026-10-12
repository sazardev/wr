package main

// Self-update. wr looks for a newer release on its own (at most once every 12
// hours, in the background, only in the interactive reader), tells you, and can
// replace itself: `wr --update`, or U inside the reader. Nothing here runs for
// --md, pipes, the MCP server or the eval, and WR_NO_UPDATE_CHECK=1 or
// update_check = false turns the background check off for good.
//
// The release comes from the GitHub API (one request to api.github.com, no
// identifier sent beyond the User-Agent). The download is the same archive and
// checksums.txt that install.sh uses: the archive's SHA-256 is checked against
// the release's checksums, the new binary must run and report the right
// version, and only then does it atomically replace the one that is running.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	releaseRepo = "sazardev/wr"
	updateEvery = 12 * time.Hour
	updateRetry = time.Hour // after a failed check, wait this long before asking again
	maxArchive  = 100 << 20
	maxBinary   = 200 << 20
)

var (
	// releaseAPI is the "latest release" endpoint; tests point it at a stub.
	releaseAPI = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
	// assetPrefix is where release assets must live to be downloaded.
	assetPrefix = "https://github.com/" + releaseRepo + "/releases/download/"
	// updateEnabled is switched on by the interactive reader only, so tests,
	// pipes, MCP and eval never make this request.
	updateEnabled  = false
	updateClient   = &http.Client{Timeout: 8 * time.Second}
	downloadClient = &http.Client{Timeout: 3 * time.Minute}
)

// releaseInfo is the part of a GitHub release wr uses.
type releaseInfo struct {
	Tag       string    `json:"tag_name"`
	Body      string    `json:"body"`
	URL       string    `json:"html_url"`
	Published time.Time `json:"published_at"`
	Draft     bool      `json:"draft,omitempty"`
	Pre       bool      `json:"prerelease,omitempty"`
	Assets    []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// updateState is what wr remembers between runs (state dir, update.json).
type updateState struct {
	Checked  time.Time    `json:"checked"`            // last successful check
	Failed   time.Time    `json:"failed,omitempty"`   // last failed check
	Latest   *releaseInfo `json:"latest,omitempty"`   // the newest release seen
	Notified string       `json:"notified,omitempty"` // the version already announced
	Seen     string       `json:"seen,omitempty"`     // the version that ran last
}

func updateStatePath() (string, error) { return statePath("update.json") }

func loadUpdateState() updateState {
	var s updateState
	if p, err := updateStatePath(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &s)
		}
	}
	return s
}

func (s updateState) save() {
	if ensureStateDir() != nil {
		return
	}
	p, err := updateStatePath()
	if err != nil {
		return
	}
	if b, err := json.Marshal(s); err == nil {
		_ = writeAtomic(p, b)
	}
}

// ---- versions ----

type semver struct {
	major, minor, patch int
	pre                 bool // a pre-release or a build after the tag (v0.5.2-0.2026…)
}

// parseSemver reads v1.2.3, 1.2.3-rc1, v0.5.2-0.20261012…+dirty. "dev" and
// anything else that is not three numbers is not comparable.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return semver{}, false
		}
		n[i] = v
	}
	return semver{n[0], n[1], n[2], hasPre && pre != ""}, true
}

// isNewer reports whether latest is a later release than current. A build that
// is only a pre-release of the same number is older than the release.
func isNewer(latest, current string) bool {
	l, ok1 := parseSemver(latest)
	c, ok2 := parseSemver(current)
	if !ok1 || !ok2 || l.pre {
		return false
	}
	switch {
	case l.major != c.major:
		return l.major > c.major
	case l.minor != c.minor:
		return l.minor > c.minor
	case l.patch != c.patch:
		return l.patch > c.patch
	}
	return c.pre
}

// shortVersion is the version as the interface shows it: v0.6.0, or v0.5.2-dev
// for a build between releases, or dev when it carries no version at all.
func shortVersion() string {
	v, ok := parseSemver(versionString())
	switch {
	case !ok:
		return "dev"
	case v.pre:
		return fmt.Sprintf("v%d.%d.%d-dev", v.major, v.minor, v.patch)
	}
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

// updatesPossible reports whether this process should look for updates at all.
func updatesPossible() bool {
	if !updateEnabled || os.Getenv("WR_NO_UPDATE_CHECK") != "" {
		return false
	}
	_, ok := parseSemver(versionString())
	return ok
}

// ---- checking ----

// fetchLatest asks the API for the latest published release.
func fetchLatest() (*releaseInfo, error) {
	req, err := http.NewRequest("GET", releaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := updateClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	var rel releaseInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.Tag == "" || rel.Draft {
		return nil, errors.New("no published release found")
	}
	return &rel, nil
}

// checkResult is what a check found.
type checkResult struct {
	Latest     *releaseInfo // the newest release known (nil if never reached)
	Newer      bool         // Latest is newer than the running version
	JustUpdate *releaseInfo // this run is the first after an update: its release notes
	Announce   bool         // Newer, and this version has not been announced yet
	Err        error
}

// checkForUpdate looks for a newer release. Unless force is set it trusts the
// last answer for updateEvery, so most launches make no request at all.
func checkForUpdate(force bool) checkResult {
	st := loadUpdateState()
	cur := versionString()
	var res checkResult

	if !force && st.Latest != nil && time.Since(st.Checked) < updateEvery {
		// fresh enough: no request
	} else if !force && !st.Failed.IsZero() && time.Since(st.Failed) < updateRetry && st.Latest != nil {
		// recently failed: do not hammer
	} else {
		rel, err := fetchLatest()
		if err != nil {
			st.Failed = time.Now()
			st.save()
			res.Err = err
			res.Latest = st.Latest
			res.Newer = st.Latest != nil && isNewer(st.Latest.Tag, cur)
			return res
		}
		st.Latest, st.Checked, st.Failed = rel, time.Now(), time.Time{}
	}

	res.Latest = st.Latest
	if st.Latest != nil {
		res.Newer = isNewer(st.Latest.Tag, cur)
		res.Announce = res.Newer && st.Notified != st.Latest.Tag
		// first run after an update: the release we are on is the one we saw
		if st.Seen != "" && st.Seen != cur && sameVersion(st.Latest.Tag, cur) {
			res.JustUpdate = st.Latest
		}
	}
	st.Seen = cur
	st.save()
	return res
}

// markAnnounced records that a version was shown to the user.
func markAnnounced(tag string) {
	st := loadUpdateState()
	st.Notified = tag
	st.save()
}

func sameVersion(a, b string) bool {
	x, ok1 := parseSemver(a)
	y, ok2 := parseSemver(b)
	return ok1 && ok2 && !x.pre && !y.pre && x == y
}

// ---- applying ----

// platformAsset is the release archive for this system.
func platformAsset() string { return fmt.Sprintf("wr_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH) }

func (r *releaseInfo) asset(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// download fetches a release asset into memory, refusing anything that does
// not live under the project's own release URL or is larger than max.
func download(url string, max int64) ([]byte, error) {
	if !strings.HasPrefix(url, assetPrefix) {
		return nil, fmt.Errorf("refusing to download from %s", url)
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("download is larger than expected")
	}
	return b, nil
}

// checksumFor reads the hex SHA-256 listed for name in a checksums.txt.
func checksumFor(sums []byte, name string) string {
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			return strings.ToLower(f[0])
		}
	}
	return ""
}

// extractBinary pulls the "wr" executable out of a release archive into dst.
func extractBinary(archive []byte, dst *os.File) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return errors.New("the archive has no wr binary")
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || path.Base(h.Name) != "wr" {
			continue
		}
		n, err := io.Copy(dst, io.LimitReader(tr, maxBinary+1))
		if err != nil {
			return err
		}
		if n > maxBinary {
			return errors.New("the binary is larger than expected")
		}
		return nil
	}
}

// currentExecutable is the file to replace: the running binary, symlinks resolved.
func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// applyUpdate downloads release rel and replaces the running binary with it.
// say reports the steps. On any failure the installed binary is untouched.
func applyUpdate(rel *releaseInfo, say func(string)) error {
	return applyUpdateTo(rel, "", say)
}

// applyUpdateTo is applyUpdate with the target path injectable (tests); an
// empty target means the running binary.
func applyUpdateTo(rel *releaseInfo, target string, say func(string)) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("no release builds for %s: update from source or WSL", runtime.GOOS)
	}
	archiveName := platformAsset()
	archiveURL, sumsURL := rel.asset(archiveName), rel.asset("checksums.txt")
	if archiveURL == "" || sumsURL == "" {
		return fmt.Errorf("release %s has no build for %s/%s", rel.Tag, runtime.GOOS, runtime.GOARCH)
	}
	if target == "" {
		exe, err := currentExecutable()
		if err != nil {
			return err
		}
		target = exe
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".wr-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: reinstall with `curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | sh`, or run `sudo wr --update`", dir, releaseRepo)
	}
	defer os.Remove(tmp.Name()) // a no-op once it was renamed into place
	defer tmp.Close()

	say("downloading " + rel.Tag + "…")
	archive, err := download(archiveURL, maxArchive)
	if err != nil {
		return err
	}
	sums, err := download(sumsURL, 1<<20)
	if err != nil {
		return err
	}
	want := checksumFor(sums, archiveName)
	if want == "" {
		return fmt.Errorf("no checksum listed for %s", archiveName)
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return errors.New("checksum mismatch: the download is corrupt, nothing was changed")
	}

	say("verifying…")
	if err := extractBinary(archive, tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	// the new binary must run, and be the version it claims to be
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, tmp.Name(), "--version").Output()
	if err != nil {
		return fmt.Errorf("the new binary does not run here: %w", err)
	}
	if !strings.Contains(string(out), strings.TrimPrefix(rel.Tag, "v")) {
		return fmt.Errorf("the new binary reports %q, expected %s", strings.TrimSpace(string(out)), rel.Tag)
	}

	say("installing…")
	if err := os.Rename(tmp.Name(), target); err != nil {
		return err
	}
	return nil
}

// ---- release notes ----

type noteKind int

const (
	noteText  noteKind = iota
	noteHead           // "Features", "Bug Fixes"
	noteItem           // a bullet
	noteTitle          // the release's own heading: text is its date (may be empty)
)

type noteLine struct {
	kind noteKind
	text string
}

var (
	noteTitleRe = regexp.MustCompile(`^#{1,2}\s+\[?v?([0-9][^\]\s)]*)\]?(?:\([^)]*\))?\s*(?:\(([\d-]+)\))?`)
	noteHeadRe  = regexp.MustCompile(`^#{3,6}\s+(.*)$`)
	noteItemRe  = regexp.MustCompile(`^\s*[*+-]\s+(.*)$`)
	noteHashRe  = regexp.MustCompile(`\s*\(\[[0-9a-f]{7,40}\]\([^)]*\)\)\s*$`)
	noteScopeRe = regexp.MustCompile(`^\*\*([^*]+):\*\*\s*`)
)

// releaseNotes turns a release body (release-please's changelog section) into
// plain lines: the title, section names and bullets, without commit links.
func releaseNotes(body string) []noteLine {
	var out []noteLine
	for _, raw := range strings.Split(strings.ReplaceAll(body, "\r", ""), "\n") {
		line := strings.TrimRight(raw, " \t")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, ":robot:") || strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "This PR was generated") {
			continue
		}
		switch {
		case noteTitleRe.MatchString(line):
			out = append(out, noteLine{noteTitle, noteTitleRe.FindStringSubmatch(line)[2]})
		case noteHeadRe.MatchString(line):
			out = append(out, noteLine{noteHead, strings.TrimSpace(noteHeadRe.FindStringSubmatch(line)[1])})
		case noteItemRe.MatchString(line):
			t := noteItemRe.FindStringSubmatch(line)[1]
			t = noteHashRe.ReplaceAllString(t, "")
			t = noteScopeRe.ReplaceAllString(t, "$1: ")
			out = append(out, noteLine{noteItem, plainMarkdown(t)})
		default:
			out = append(out, noteLine{noteText, plainMarkdown(strings.TrimSpace(line))})
		}
	}
	return out
}

var mdInline = regexp.MustCompile("\\[([^\\]]*)\\]\\([^)]*\\)|\\*\\*|`")

// plainMarkdown drops links (keeping their text), bold markers and backticks.
func plainMarkdown(s string) string {
	return sanitize(mdInline.ReplaceAllString(s, "$1"))
}

// runUpdateCLI is `wr --update`: look for a newer release and install it.
func runUpdateCLI(out io.Writer) error {
	updateEnabled = true
	cur := versionString()
	if _, ok := parseSemver(cur); !ok {
		return fmt.Errorf("this build (%s) carries no release version, so there is nothing to compare: install a release with install.sh", cur)
	}
	fmt.Fprintf(out, "wr %s: looking for a newer release…\n", shortVersion())
	res := checkForUpdate(true)
	if res.Latest == nil {
		if res.Err != nil {
			return fmt.Errorf("could not check for updates: %v", shortErr(res.Err))
		}
		return errors.New("could not find a release")
	}
	if !res.Newer {
		fmt.Fprintf(out, "wr %s is up to date (latest release: %s)\n", shortVersion(), res.Latest.Tag)
		return nil
	}
	fmt.Fprintf(out, "\nwr %s is available (you have %s)\n", res.Latest.Tag, shortVersion())
	shown := 0
	for _, n := range releaseNotes(res.Latest.Body) {
		if shown >= 24 {
			fmt.Fprintf(out, "  … full notes: %s\n", res.Latest.URL)
			break
		}
		switch n.kind {
		case noteTitle:
			continue
		case noteHead:
			fmt.Fprintf(out, "\n%s\n", n.text)
		case noteItem:
			fmt.Fprintf(out, "  • %s\n", n.text)
		default:
			fmt.Fprintf(out, "  %s\n", n.text)
		}
		shown++
	}
	fmt.Fprintln(out)
	if err := applyUpdate(res.Latest, func(s string) { fmt.Fprintln(out, s) }); err != nil {
		return err
	}
	markAnnounced(res.Latest.Tag)
	fmt.Fprintf(out, "✓ wr %s installed. Run wr again to use it.\n", res.Latest.Tag)
	return nil
}
