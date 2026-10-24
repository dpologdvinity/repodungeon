package dungeon

import (
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/dpologdvinity/repodungeon/internal/repo"
)

const (
	MaxDepth = 7 // deeper directories fold into their depth-7 ancestor
	MaxRooms = 9 // rooms per level (3x3 grid)
)

type RoomSpec struct {
	Name  string
	Files []repo.File // sorted by Path
}

func depth(dir string) int {
	if dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// roomDir is the directory of p, cut to at most MaxDepth components.
func roomDir(p string) string {
	dir := path.Dir(p)
	if dir == "." {
		return ""
	}
	if parts := strings.Split(dir, "/"); len(parts) > MaxDepth {
		return strings.Join(parts[:MaxDepth], "/")
	}
	return dir
}

func parentDir(dir string) string {
	if p := path.Dir(dir); p != "." {
		return p
	}
	return ""
}

func roomName(dir string) string {
	if dir == "" {
		return "/"
	}
	return dir + "/"
}

// Plan groups text files into rooms per directory depth: [level][room].
func Plan(files []repo.File) [][]RoomSpec {
	byDir := map[string][]repo.File{}
	dirs := map[string]bool{}
	for _, f := range files {
		if !f.Text() {
			continue
		}
		d := roomDir(f.Path)
		byDir[d] = append(byDir[d], f)
		for {
			dirs[d] = true
			if d == "" {
				break
			}
			d = parentDir(d)
		}
	}
	if len(dirs) == 0 {
		return nil
	}
	maxDepth := 0
	for d := range dirs {
		maxDepth = max(maxDepth, depth(d))
	}
	levels := make([][]RoomSpec, maxDepth+1)
	for lvl := range levels {
		var ds []string
		for d := range dirs {
			if depth(d) == lvl {
				ds = append(ds, d)
			}
		}
		sort.Strings(ds)
		var misc []repo.File
		folded := len(ds) > MaxRooms
		if folded {
			ranked := slices.Clone(ds)
			sort.SliceStable(ranked, func(i, j int) bool {
				return len(byDir[ranked[i]]) > len(byDir[ranked[j]])
			})
			keep := map[string]bool{}
			for _, d := range ranked[:MaxRooms-1] {
				keep[d] = true
			}
			var kept []string
			for _, d := range ds {
				if keep[d] {
					kept = append(kept, d)
				} else {
					misc = append(misc, byDir[d]...)
				}
			}
			ds = kept
		}
		for _, d := range ds {
			levels[lvl] = append(levels[lvl], RoomSpec{Name: roomName(d), Files: sortFiles(byDir[d])})
		}
		if folded {
			levels[lvl] = append(levels[lvl], RoomSpec{Name: "misc/", Files: sortFiles(misc)})
		}
	}
	return levels
}

func sortFiles(fs []repo.File) []repo.File {
	out := slices.Clone(fs)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
