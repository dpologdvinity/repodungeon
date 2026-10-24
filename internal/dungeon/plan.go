package dungeon

import (
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/dpologdvinity/repodungeon/internal/repo"
)

const (
	// MaxDepth is the deepest directory level. Deeper directories fold into
	// their depth-7 ancestor, so a dungeon has at most 8 levels (0 to 7).
	MaxDepth = 7
	// MaxRooms is the most rooms a level can hold: one per cell of the 3x3
	// map grid. Extra directories fold into a "misc/" room.
	MaxRooms = 9
)

// RoomSpec describes one planned room before it is placed on a map.
type RoomSpec struct {
	// Name is the room title: the directory path with a trailing slash,
	// "/" for the repo root, or "misc/" for the overflow room.
	Name string
	// Files are the text files that live directly in this room's directory,
	// sorted by Path. A room can have none: a parent directory with only
	// subdirectories still gets a room so that levels are never skipped.
	Files []repo.File // sorted by Path
}

// depth returns the number of path components in dir. The repo root ("") is 0.
func depth(dir string) int {
	if dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// roomDir is the directory of p, cut to at most MaxDepth components.
//
// Files in the root give "". Deeper directories fold into their depth-7
// ancestor so the dungeon never has more than 8 levels (depth 0 to 7).
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

// parentDir returns the parent of dir, or "" (the root) at the top.
func parentDir(dir string) string {
	if p := path.Dir(dir); p != "." {
		return p
	}
	return ""
}

// roomName returns the room title for dir: "/" for the root, else dir + "/".
func roomName(dir string) string {
	if dir == "" {
		return "/"
	}
	return dir + "/"
}

// Plan groups text files into rooms per directory depth: [level][room].
//
// Level i holds the rooms for directories at depth i. Every ancestor of a
// directory with files also gets a room, so levels are never skipped; such
// rooms may hold no files. Binary files are ignored. If no text file exists
// it returns nil.
//
// A level may hold at most MaxRooms rooms (the 3x3 grid). When a level has
// more directories, the MaxRooms-1 busiest ones (most files, ties by name)
// keep their own room and all the others fold into one trailing "misc/" room.
// Rooms are otherwise sorted by name. See docs/SPEC.md section 5.2.
func Plan(files []repo.File) [][]RoomSpec {
	byDir := map[string][]repo.File{}
	dirs := map[string]bool{}
	for _, f := range files {
		if !f.Text() {
			continue
		}
		d := roomDir(f.Path)
		byDir[d] = append(byDir[d], f)
		// Mark the directory and every ancestor up to the root.
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
		// Map iteration order is random, so sort for determinism.
		sort.Strings(ds)
		var misc []repo.File
		folded := len(ds) > MaxRooms
		if folded {
			// Rank by file count. The stable sort keeps name order on ties.
			ranked := slices.Clone(ds)
			sort.SliceStable(ranked, func(i, j int) bool {
				return len(byDir[ranked[i]]) > len(byDir[ranked[j]])
			})
			keep := map[string]bool{}
			for _, d := range ranked[:MaxRooms-1] {
				keep[d] = true
			}
			// Walk ds (name order) so kept rooms stay sorted by name.
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

// sortFiles returns a copy of fs sorted by Path. The input is not changed.
func sortFiles(fs []repo.File) []repo.File {
	out := slices.Clone(fs)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
