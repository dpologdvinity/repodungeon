// Package repo reads a git repository's HEAD tree, contents and churn.
package repo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// MaxBlob is the largest blob that is read; bigger ones are Skipped.
const MaxBlob = 1 << 20

// Sentinel errors returned by Scan. Callers compare them with errors.Is.
var (
	// ErrGitMissing means the git program is not on PATH.
	ErrGitMissing = errors.New("git not found in PATH")
	// ErrNotRepo means the directory is not inside a git repository.
	ErrNotRepo = errors.New("not a git repository")
	// ErrNoCommits means the repository exists but HEAD points to nothing.
	ErrNoCommits = errors.New("repository has no commits")
)

// GitError is any other failing git call.
type GitError struct {
	// Args are the git arguments, without the leading "-C dir". Stderr is the
	// text git wrote to standard error.
	Args   []string
	Stderr string
}

// Error returns "git <args>: <first line of stderr>". Only the first line
// is kept so the message fits on one terminal line.
func (e *GitError) Error() string {
	first, _, _ := strings.Cut(strings.TrimSpace(e.Stderr), "\n")
	return "git " + strings.Join(e.Args, " ") + ": " + first
}

// File is one committed file at HEAD and the numbers measured for it.
type File struct {
	// Path is repo-relative with forward slashes. Scan replaces unprintable
	// characters with '?' and keeps paths unique, because the path is shown
	// in the terminal and a hostile repo must not inject escape codes.
	Path string
	// Lines is the number of lines; a last line without a newline counts, and
	// binary or skipped files have 0. Churn is how many recent commits
	// changed the file.
	Lines, Churn int
	// Todos and Fixmes count whole-word TODO and FIXME matches.
	Todos, Fixmes int
	// Binary is true if a NUL byte appears in the first 8000 bytes.
	Binary bool
	// Skipped is true if the content was not read: the blob is larger than
	// MaxBlob, or it is missing from a partial clone.
	Skipped bool
}

// Text reports whether the file can become a monster, loot or the boss.
func (f File) Text() bool { return !f.Binary && !f.Skipped }

// Repo is the result of Scan.
type Repo struct {
	// Name is the base name of the top-level directory. Head is the full hash
	// of the HEAD commit.
	Name, Head string
	// Files are the committed regular files, sorted by Path.
	Files []File // sorted by Path
}

// todoRe and fixmeRe match the markers as whole words, so "TODOS" does not
// count.
var (
	todoRe  = regexp.MustCompile(`\bTODO\b`)
	fixmeRe = regexp.MustCompile(`\bFIXME\b`)
)

// command builds a git command that runs in dir.
// LC_ALL=C gives stable, untranslated output. GIT_NO_LAZY_FETCH=1 stops git
// from fetching a missing blob over the network, one blob at a time, in a
// partial clone. That would be very slow and could contact a remote the user
// did not expect. A missing blob is reported instead and Scan skips it.
func command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_NO_LAZY_FETCH=1") // partial clones: never fetch blobs one by one
	return cmd
}

// git runs git in dir and returns its standard output. On failure it
// returns a *GitError that carries the captured standard error.
func git(dir string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := command(dir, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, &GitError{Args: args, Stderr: stderr.String()}
	}
	return out, nil
}

// Scan reads the committed files of the repository at dir.
// It only looks at the HEAD commit, never at the work tree, so ignored and
// untracked files are not seen. It returns ErrGitMissing, ErrNotRepo or
// ErrNoCommits for the common problems and a *GitError for other git
// failures.
func Scan(dir string) (*Repo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, ErrGitMissing
	}
	top, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, ErrNotRepo
	}
	r := &Repo{Name: filepath.Base(strings.TrimSpace(string(top)))}
	head, err := git(dir, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		return nil, ErrNoCommits
	}
	r.Head = strings.TrimSpace(string(head))

	tree, err := git(dir, "ls-tree", "-r", "-z", "-l", "--full-tree", "HEAD")
	if err != nil {
		return nil, err
	}
	// ls-tree -z output is NUL-separated entries: "<mode> <type> <hash> <size>\t<path>".
	// -z keeps odd file names (spaces, quotes, newlines) unescaped.
	hashOf := map[string]string{}
	for _, ent := range strings.Split(string(tree), "\x00") {
		meta, path, ok := strings.Cut(ent, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		// Keep regular blobs only. Drop gitlinks (type commit) and symlinks
		// (mode 120000), whose "content" is just a target path.
		if len(f) != 4 || f[1] != "blob" || f[0] == "120000" {
			continue
		}
		size, _ := strconv.Atoi(f[3])
		hashOf[path] = f[2]
		r.Files = append(r.Files, File{Path: path, Skipped: size > MaxBlob})
	}
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
	// idx maps path to position in r.Files. hashes and readIdx are parallel
	// lists: the blob to ask for, and the file it belongs to. Oversize files
	// are left out, so they are never read.
	idx := map[string]int{}
	var hashes []string
	var readIdx []int
	for i, f := range r.Files {
		idx[f.Path] = i
		if !f.Skipped {
			hashes = append(hashes, hashOf[f.Path])
			readIdx = append(readIdx, i)
		}
	}
	if err := readBlobs(dir, r, hashes, readIdx); err != nil {
		return nil, err
	}

	// Churn: count how many of the last 5000 non-merge commits touched each
	// file. Renames are off so each path counts on its own.
	log, err := git(dir, "log", "-z", "--no-merges", "--no-renames", "--max-count=5000",
		"--format=%x1e", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	// Each commit starts with the \x1e marker from --format. -z separates the
	// file names with NUL. We use \x1e and NUL because they cannot appear in
	// a path the way newlines can, so the split is always exact. The first
	// name after a marker may carry a leading newline, which is trimmed.
	for _, chunk := range strings.Split(string(log), "\x1e") {
		for _, p := range strings.Split(chunk, "\x00") {
			p = strings.TrimPrefix(p, "\n")
			if i, ok := idx[p]; ok && p != "" {
				r.Files[i].Churn++
			}
		}
	}
	// This runs after the churn lookup, which needs the original paths.
	// Paths come from untrusted repos and reach the terminal: neutralize
	// control characters, and keep paths unique when that makes two collide.
	taken := map[string]bool{}
	for i := range r.Files {
		base := strings.Map(func(c rune) rune {
			if unicode.IsPrint(c) {
				return c
			}
			return '?'
		}, r.Files[i].Path)
		p := base
		for n := 2; taken[p]; n++ {
			p = fmt.Sprintf("%s~%d", base, n)
		}
		taken[p] = true
		r.Files[i].Path = p
	}
	return r, nil
}

// readBlobs fills in Lines, Todos, Fixmes and Binary for the files at
// readIdx, whose blob hashes are in hashes (same order).
//
// It starts one "git cat-file --batch" process and streams all hashes
// through it. That is far faster than starting one git process per file.
// For each hash, git prints a header line "<hash> blob <size>", then <size>
// bytes of content, then one newline. A blob that is not available prints
// "<hash> missing"; that file is marked Skipped.
func readBlobs(dir string, r *Repo, hashes []string, readIdx []int) error {
	if len(hashes) == 0 {
		return nil
	}
	cmd := command(dir, "cat-file", "--batch")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Write the requests from a goroutine. If we wrote them all before
	// reading, git could fill its output pipe and block while we block on
	// the input pipe: a deadlock. Closing stdin tells git there is no more.
	go func() {
		w := bufio.NewWriter(in)
		for _, h := range hashes {
			_, _ = w.WriteString(h + "\n") // a write error means git died; the reader reports it
		}
		_ = w.Flush()
		_ = in.Close()
	}()
	br := bufio.NewReader(out)
	// fail stops git and reports the problem. Git's own stderr is the best
	// message; the Go error is the fallback.
	fail := func(e error) error {
		_ = cmd.Process.Kill() // already failing; the original error is what matters
		_ = cmd.Wait()
		msg := stderr.String()
		if msg == "" {
			msg = e.Error()
		}
		return &GitError{Args: []string{"cat-file", "--batch"}, Stderr: msg}
	}
	for _, i := range readIdx {
		header, err := br.ReadString('\n')
		if err != nil {
			return fail(err)
		}
		hf := strings.Fields(header)
		if len(hf) == 2 && hf[1] == "missing" { // blob not present locally (partial clone)
			r.Files[i].Skipped = true
			continue
		}
		if len(hf) != 3 || hf[1] != "blob" {
			return fail(errors.New("unexpected cat-file header: " + strings.TrimSpace(header)))
		}
		size, err := strconv.Atoi(hf[2])
		if err != nil {
			return fail(err)
		}
		// Read exactly size bytes, then the single newline git adds after them.
		body := make([]byte, size)
		if _, err := io.ReadFull(br, body); err != nil {
			return fail(err)
		}
		if _, err := br.ReadByte(); err != nil {
			return fail(err)
		}
		// A NUL in the first 8000 bytes marks the file as binary.
		f := &r.Files[i]
		if bytes.IndexByte(body[:min(size, 8000)], 0) >= 0 {
			f.Binary = true
			continue
		}
		// Count newlines, plus one for a final line that lacks a newline.
		f.Lines = bytes.Count(body, []byte{'\n'})
		if size > 0 && body[size-1] != '\n' {
			f.Lines++
		}
		// The substring check skips the slower regexp for most files.
		if bytes.Contains(body, []byte("TODO")) {
			f.Todos = len(todoRe.FindAllIndex(body, -1))
		}
		if bytes.Contains(body, []byte("FIXME")) {
			f.Fixmes = len(fixmeRe.FindAllIndex(body, -1))
		}
	}
	if err := cmd.Wait(); err != nil {
		return &GitError{Args: []string{"cat-file", "--batch"}, Stderr: stderr.String() + err.Error()}
	}
	return nil
}
