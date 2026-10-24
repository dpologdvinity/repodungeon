// Package repo reads a git repository's HEAD tree, contents and churn.
package repo

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MaxBlob is the largest blob that is read; bigger ones are Skipped.
const MaxBlob = 1 << 20

var (
	ErrGitMissing = errors.New("git not found in PATH")
	ErrNotRepo    = errors.New("not a git repository")
	ErrNoCommits  = errors.New("repository has no commits")
)

// GitError is any other failing git call.
type GitError struct {
	Args   []string
	Stderr string
}

func (e *GitError) Error() string {
	first, _, _ := strings.Cut(strings.TrimSpace(e.Stderr), "\n")
	return "git " + strings.Join(e.Args, " ") + ": " + first
}

type File struct {
	Path          string // repo-relative, forward slashes
	Lines, Churn  int
	Todos, Fixmes int
	Binary        bool
	Skipped       bool
}

// Text reports whether the file can become a monster, loot or the boss.
func (f File) Text() bool { return !f.Binary && !f.Skipped }

type Repo struct {
	Name, Head string
	Files      []File // sorted by Path
}

var (
	todoRe  = regexp.MustCompile(`\bTODO\b`)
	fixmeRe = regexp.MustCompile(`\bFIXME\b`)
)

func command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd
}

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
	hashOf := map[string]string{}
	for _, ent := range strings.Split(string(tree), "\x00") {
		meta, path, ok := strings.Cut(ent, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) != 4 || f[1] != "blob" || f[0] == "120000" {
			continue
		}
		size, _ := strconv.Atoi(f[3])
		hashOf[path] = f[2]
		r.Files = append(r.Files, File{Path: path, Skipped: size > MaxBlob})
	}
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
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

	log, err := git(dir, "log", "-z", "--no-merges", "--no-renames", "--max-count=5000",
		"--format=%x1e", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	for _, chunk := range strings.Split(string(log), "\x1e") {
		for _, p := range strings.Split(chunk, "\x00") {
			p = strings.TrimPrefix(p, "\n")
			if i, ok := idx[p]; ok && p != "" {
				r.Files[i].Churn++
			}
		}
	}
	return r, nil
}

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
	go func() {
		w := bufio.NewWriter(in)
		for _, h := range hashes {
			w.WriteString(h + "\n")
		}
		w.Flush()
		in.Close()
	}()
	br := bufio.NewReader(out)
	fail := func(e error) error {
		cmd.Process.Kill()
		cmd.Wait()
		if stderr.Len() > 0 {
			return &GitError{Args: []string{"cat-file", "--batch"}, Stderr: stderr.String()}
		}
		return e
	}
	for _, i := range readIdx {
		header, err := br.ReadString('\n')
		if err != nil {
			return fail(err)
		}
		hf := strings.Fields(header)
		if len(hf) != 3 || hf[1] != "blob" {
			return fail(errors.New("unexpected cat-file header: " + strings.TrimSpace(header)))
		}
		size, err := strconv.Atoi(hf[2])
		if err != nil {
			return fail(err)
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(br, body); err != nil {
			return fail(err)
		}
		if _, err := br.ReadByte(); err != nil {
			return fail(err)
		}
		f := &r.Files[i]
		if bytes.IndexByte(body[:min(size, 8000)], 0) >= 0 {
			f.Binary = true
			continue
		}
		f.Lines = bytes.Count(body, []byte{'\n'})
		if size > 0 && body[size-1] != '\n' {
			f.Lines++
		}
		f.Todos = len(todoRe.FindAllIndex(body, -1))
		f.Fixmes = len(fixmeRe.FindAllIndex(body, -1))
	}
	return cmd.Wait()
}
