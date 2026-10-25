// Package game is the pure roguelike state machine, driven by key strings.
//
// It has no Bubble Tea, terminal or clock dependency. State.Apply takes one
// key string (as Bubble Tea's KeyPressMsg.String() would give it) and changes
// the state. State.Render and State.Card turn the state into plain text.
// Rules are specified in docs/SPEC.md sections 6 and 7.
package game

import (
	"fmt"
	"slices"

	"github.com/dpologdvinity/repodungeon/internal/dungeon"
)

// Mode is the input mode of the game. It decides how keys are read.
type Mode int

const (
	// Play is the normal mode: movement, waiting, stairs and opening the inventory.
	Play Mode = iota
	// Inventory is the item menu. Keys a-j use items, esc or i closes it.
	Inventory
	// Help is the how-to-play screen. Any key except q closes it.
	Help
)

// Outcome says whether the run is still going and, if not, how it ended.
type Outcome int

const (
	// Running means the run is not over yet. It is the zero value.
	Running Outcome = iota
	// Won means the player slew the boss (SPEC 7.3).
	Won
	// Died means the player's HP reached 0 in the monster phase.
	Died
	// Quit means the player pressed q or ctrl+c.
	Quit
)

const (
	// packSize is the maximum number of items in the pack (SPEC 7.5).
	packSize = 10
	// fovRadius is the field-of-view radius in tiles (SPEC 7.4).
	fovRadius = 5
)

// Player is the hero: position, stats and the pack.
type Player struct {
	// Pos is the player's tile on the current level.
	Pos dungeon.Point
	// HP is current health, MaxHP its cap. ATK is attack strength. Level and XP track progress (SPEC 7.3).
	HP, MaxHP, ATK, Level, XP int
	// GitEnergy is tactical ability energy (max 100, starts at 50).
	GitEnergy int
	// Inv is the pack, in pickup order. Using an item keeps the order of the rest.
	Inv []dungeon.Item
	// Weapon is the equipped weapon, or nil if unarmed.
	Weapon *dungeon.Item
	// StashTurns is the remaining duration of git stash stealth.
	StashTurns int
	// LastDamageTaken is damage suffered in the most recent monster phase.
	LastDamageTaken int
}

// State is the whole game. Apply is the only way to change it from outside.
// It has no I/O and no clock, so tests can drive it with key strings.
type State struct {
	// D is the generated dungeon. Its levels, items and monsters are mutated during play.
	D *dungeon.Dungeon
	// Depth is the zero-based index of the current level.
	Depth int // index into D.Levels
	// P is the player.
	P Player
	// Mode is the current input mode.
	Mode Mode
	// Outcome is Running until the run ends. After that Apply ignores all keys.
	Outcome Outcome
	// Turn counts turns that took time. Waiting, moving, attacking, using items and descending count.
	Turn int
	// Msgs are the messages from the last key press. Apply clears them first.
	Msgs []string
	// Kills is the number of monsters slain.
	Kills int
	// KilledBy is the name of the monster that killed the player. Empty unless Outcome is Died.
	KilledBy string
	// LootTodo and LootFixme count potions and scrolls picked up, for the card (SPEC 7.8).
	LootTodo, LootFixme int // items picked up
	// restWaitCount counts consecutive safe resting turns waiting with ".".
	restWaitCount int
	// lastDir is the delta of the player's most recent movement or attack key.
	lastDir dungeon.Point
	// seen holds one flat W*H grid per level. A true tile was in view at some point and is drawn from memory.
	seen [][]bool
	// helpPage is the page of the help screen being shown (0 or 1). It only matters in Help mode.
	helpPage int
	// rng is the combat RNG. It is seeded from the dungeon seed, so a run is reproducible from the key list.
	rng *dungeon.RNG // combat RNG
}

// New starts a run on level 0 of d with the starting stats (30 HP, 3 ATK, level 1, 50 energy).
// It computes the first field of view, so monsters in sight are awake from turn 0.
// d must have at least one level.
func New(d *dungeon.Dungeon) *State {
	s := &State{D: d, rng: dungeon.NewRNG(d.Seed + 1)}
	s.P = Player{Pos: d.Levels[0].Start, HP: 30, MaxHP: 30, ATK: 3, Level: 1, GitEnergy: 50}
	for _, l := range d.Levels {
		s.seen = append(s.seen, make([]bool, l.W*l.H))
	}
	s.updateFOV()
	s.P.GitEnergy = 50
	s.Msgs = []string{fmt.Sprintf("Welcome to %s! Slay %s on depth %d.", d.Repo, d.BossName, len(d.Levels))}
	return s
}

// OpenHelp shows the how-to-play screen. The command line calls it before
// the first frame so a new player reads the rules before the map. "?" opens it
// again during play. It costs no turn.
func (s *State) OpenHelp() { s.Mode, s.helpPage = Help, 0 }

// level returns the level the player is on.
func (s *State) level() *dungeon.Level { return s.D.Levels[s.Depth] }

// Done reports whether the run is over (won, died or quit).
func (s *State) Done() bool { return s.Outcome != Running }

// Visible reports whether p is within the field-of-view radius (walls do not block).
func (s *State) Visible(p dungeon.Point) bool {
	dx, dy := p.X-s.P.Pos.X, p.Y-s.P.Pos.Y
	return dx*dx+dy*dy <= fovRadius*fovRadius
}

// updateFOV marks every tile in view as seen and wakes every living monster in view.
// It scans only the bounding square of the radius and clips it to the map.
// Newly revealed floor tiles grant +1 GitEnergy (capped at 100).
// Monsters never go back to sleep (SPEC 7.4).
func (s *State) updateFOV() {
	l := s.level()
	for y := max(0, s.P.Pos.Y-fovRadius); y <= min(l.H-1, s.P.Pos.Y+fovRadius); y++ {
		for x := max(0, s.P.Pos.X-fovRadius); x <= min(l.W-1, s.P.Pos.X+fovRadius); x++ {
			if s.Visible(dungeon.Point{X: x, Y: y}) {
				idx := y*l.W + x
				if !s.seen[s.Depth][idx] {
					s.seen[s.Depth][idx] = true
					if l.Tiles[idx] == dungeon.Floor {
						s.P.GitEnergy = min(100, s.P.GitEnergy+1)
					}
				}
			}
		}
	}
	for i := range l.Monsters {
		if m := &l.Monsters[i]; m.HP > 0 && s.Visible(m.Pos) {
			m.Awake = true
		}
	}
	if s.hasAwakeMonstersInFOV() {
		s.restWaitCount = 0
	}
}

// hasAwakeMonstersInFOV reports whether any awake, living monster is in the player's FOV.
func (s *State) hasAwakeMonstersInFOV() bool {
	l := s.level()
	for i := range l.Monsters {
		m := &l.Monsters[i]
		if m.HP > 0 && m.Awake && s.Visible(m.Pos) {
			return true
		}
	}
	return false
}

// safeRestTurn handles natural healing when waiting safely without awake monsters in FOV.
// Every 4 safe wait turns restores 1 HP up to MaxHP.
func (s *State) safeRestTurn() {
	if s.Done() || s.P.HP <= 0 || s.hasAwakeMonstersInFOV() || s.P.HP >= s.P.MaxHP {
		s.restWaitCount = 0
		return
	}
	s.restWaitCount++
	if s.restWaitCount%4 == 0 {
		s.P.HP = min(s.P.MaxHP, s.P.HP+1)
		s.say("You rest quietly. +1 HP.")
	}
}

// say appends a formatted message to Msgs.
func (s *State) say(format string, a ...any) { s.Msgs = append(s.Msgs, fmt.Sprintf(format, a...)) }

// moves maps movement keys (vi keys and arrow names) to a tile delta.
var moves = map[string]dungeon.Point{
	"h": {X: -1}, "left": {X: -1}, "l": {X: 1}, "right": {X: 1},
	"k": {Y: -1}, "up": {Y: -1}, "j": {Y: 1}, "down": {Y: 1},
}

// Apply handles one key press. Keys after the game is over do nothing.
//
// Turn order (SPEC 7.2): the player acts, then, if the action took a turn
// and the run is still going, the monsters act. Then Turn increases and the
// field of view is updated. An action that takes no turn (a wall bump, an
// unknown key, opening the menu) skips all of that. Msgs is cleared on every
// key, so it only ever holds the messages of the last key.
func (s *State) Apply(key string) {
	if s.Done() {
		return
	}
	s.Msgs = nil
	if key == "q" || key == "ctrl+c" {
		s.Outcome = Quit
		return
	}
	var turn bool
	switch s.Mode {
	case Help:
		// Any key turns the page, and on the last page closes help. It does nothing else.
		if s.helpPage++; s.helpPage >= helpPages {
			s.Mode = Play
		}
	case Inventory:
		turn = s.inventoryKey(key)
	default:
		turn = s.playKey(key)
	}
	if !turn {
		return
	}
	if s.Outcome == Running {
		s.monsterPhase()
	}
	if s.P.StashTurns > 0 && key != "1" {
		s.P.StashTurns--
	}
	s.Turn++
	s.updateFOV()
}

// playKey runs a Play-mode key and reports whether it took a turn.
// Moves, Git Powers, and "." take a turn. "i" and ">" never return true here:
// descend does its own turn accounting (see descend).
func (s *State) playKey(key string) bool {
	if d, ok := moves[key]; ok {
		s.restWaitCount = 0
		s.lastDir = d
		return s.move(d)
	}
	switch key {
	case "1", "2", "3":
		s.restWaitCount = 0
		return s.castGitPower(key)
	case ".":
		s.safeRestTurn()
		return true
	case "i":
		s.Mode = Inventory
	case "?":
		s.OpenHelp()
	case ">":
		s.restWaitCount = 0
		s.descend()
	}
	return false
}

// inventoryKey runs an Inventory-mode key and reports whether it took a turn.
// Only using an item takes a turn. Closing the menu, or pressing a letter
// with no item behind it, does not (SPEC 7.1).
func (s *State) inventoryKey(key string) bool {
	switch {
	case key == "esc" || key == "i":
		s.Mode = Play
	case len(key) == 1 && key[0] >= 'a' && key[0] <= 'j' && int(key[0]-'a') < len(s.P.Inv):
		s.restWaitCount = 0
		s.use(int(key[0] - 'a'))
		s.Mode = Play
		return true
	}
	return false
}

// equipWeapon equips the weapon at pack index i into the active weapon slot.
// If a weapon was already equipped, it is swapped back into pack index i;
// otherwise index i is removed from the pack.
func (s *State) equipWeapon(i int) {
	w := s.P.Inv[i]
	wPtr := new(dungeon.Item)
	*wPtr = w
	if s.P.Weapon != nil {
		old := *s.P.Weapon
		s.P.Weapon = wPtr
		s.P.Inv[i] = old
	} else {
		s.P.Inv = slices.Delete(s.P.Inv, i, i+1)
		s.P.Weapon = wPtr
	}
	s.say("You equip the %s.", w.Name())
}

// use consumes or equips the pack item at index i. A potion heals up to 10 HP without
// exceeding MaxHP. A scroll adds 1 ATK. A weapon is equipped into the active
// weapon slot. Order of the other items is kept (SPEC 7.5).
func (s *State) use(i int) {
	it := s.P.Inv[i]
	if it.Kind == dungeon.Weapon {
		s.equipWeapon(i)
		return
	}
	s.P.Inv = slices.Delete(s.P.Inv, i, i+1)
	if it.Kind == dungeon.Potion {
		heal := min(10, s.P.MaxHP-s.P.HP)
		s.P.HP += heal
		s.say("You drink a TODO potion. +%d HP.", heal)
		return
	}
	s.P.ATK++
	s.say("You read a FIXME scroll. ATK +1.")
}

// descend takes the stairs; it costs a turn but monsters do not act.
//
// It increments Turn itself and returns nothing, so playKey reports no turn
// and Apply skips the monster phase. This is on purpose (SPEC 7.6): the
// player arrives on a new level at its Start and must not be hit by
// monsters that have had no chance to be seen. There is no way back up.
// Without stairs under the player, or on the last level, it only sets a message.
func (s *State) descend() {
	if s.level().At(s.P.Pos) != dungeon.Stairs || s.Depth+1 >= len(s.D.Levels) {
		s.say("There are no stairs here.")
		return
	}
	s.Depth++
	s.P.Pos = s.level().Start
	s.P.LastDamageTaken = 0
	if s.P.StashTurns > 0 {
		s.P.StashTurns--
	}
	s.say("You descend to depth %d.", s.Depth+1)
	s.Turn++
	s.updateFOV()
}

// pickUp moves the item at t, if any, from the level into the pack.
// A full pack leaves the item on the floor and says so. The player has
// already stepped onto t by then, and that still takes a turn (SPEC 7.5).
// If the player is unarmed, picking up a weapon auto-equips it if the pack is empty.
func (s *State) pickUp(t dungeon.Point) {
	l := s.level()
	for i, it := range l.Items {
		if it.Pos != t {
			continue
		}
		if len(s.P.Inv) >= packSize {
			s.say("Your pack is full.")
			return
		}
		l.Items = slices.Delete(l.Items, i, i+1)
		autoEquip := it.Kind == dungeon.Weapon && s.P.Weapon == nil && len(s.P.Inv) == 0
		s.P.Inv = append(s.P.Inv, it)
		s.say("You pick up a %s.", it.Kind.Name())
		switch it.Kind {
		case dungeon.Potion:
			s.LootTodo++
		case dungeon.Scroll:
			s.LootFixme++
		}
		if autoEquip {
			s.equipWeapon(0)
		}
		return
	}
}

// monsterAt returns the living monster at p, or nil.
// Dead monsters stay in the Monsters slice with HP <= 0, so they are skipped here.
func (s *State) monsterAt(p dungeon.Point) *dungeon.Monster {
	l := s.level()
	for i := range l.Monsters {
		if m := &l.Monsters[i]; m.HP > 0 && m.Pos == p {
			return m
		}
	}
	return nil
}

// MonsterAt returns the living monster at p, or nil.
func (s *State) MonsterAt(p dungeon.Point) *dungeon.Monster {
	return s.monsterAt(p)
}

// move tries to step by d and reports whether a turn passed.
// A living monster on the target tile is attacked instead of entered.
// With a spear equipped, a living monster 2 tiles away is attacked if the
// intervening tile is walkable and free of monsters.
// A non-walkable tile does nothing and costs no turn (SPEC 7.2).
func (s *State) move(d dungeon.Point) bool {
	t := dungeon.Point{X: s.P.Pos.X + d.X, Y: s.P.Pos.Y + d.Y}
	if m := s.monsterAt(t); m != nil {
		s.attack(m)
		return true
	}
	if s.spearReachAttack(d) {
		return true
	}
	if !dungeon.Walkable(s.level().At(t)) {
		return false
	}
	s.P.Pos = t
	s.pickUp(t)
	return true
}

// spearReachAttack checks if the player is wielding a spear and can perform a 2-tile
// reach attack in direction d. If the intervening tile is walkable and has no monster,
// and the 2nd tile holds a living monster, it attacks that monster without moving the player.
func (s *State) spearReachAttack(d dungeon.Point) bool {
	if s.P.Weapon == nil || s.P.Weapon.Archetype != dungeon.Spear {
		return false
	}
	t1 := dungeon.Point{X: s.P.Pos.X + d.X, Y: s.P.Pos.Y + d.Y}
	if !dungeon.Walkable(s.level().At(t1)) || s.monsterAt(t1) != nil {
		return false
	}
	t2 := dungeon.Point{X: s.P.Pos.X + 2*d.X, Y: s.P.Pos.Y + 2*d.Y}
	m := s.monsterAt(t2)
	if m == nil {
		return false
	}
	s.attack(m)
	return true
}

// hammerKnockback pushes m by vector d. If the destination tile is blocked
// by a wall, the player, or another living monster, m takes +2 collision damage.
func (s *State) hammerKnockback(m *dungeon.Monster, d dungeon.Point) {
	if m == nil || m.HP <= 0 {
		return
	}
	dest := dungeon.Point{X: m.Pos.X + d.X, Y: m.Pos.Y + d.Y}
	if dest == s.P.Pos || !dungeon.Walkable(s.level().At(dest)) || s.monsterAt(dest) != nil {
		m.HP -= 2
		s.say("%s takes 2 collision damage.", m.Name)
		if m.HP <= 0 {
			s.slay(m)
		}
		return
	}
	m.Pos = dest
}

// daggerCritical returns the damage multiplier for a dagger attack against m:
// 2 if m is asleep or the player is in git stash stealth, 1 otherwise.
func (s *State) daggerCritical(m *dungeon.Monster) int {
	if m != nil && (!m.Awake || s.P.StashTurns > 0) {
		return 2
	}
	return 1
}

// roll is the damage of an attack with strength atk: atk-1..atk+1, at least 1.
// The player and the monsters share it and draw from the one combat RNG,
// so the order of calls matters for reproducibility (SPEC 7.3).
func (s *State) roll(atk int) int { return max(1, atk+s.rng.IntN(3)-1) }

// slay marks m as killed, credits XP, and handles boss defeat.
func (s *State) slay(m *dungeon.Monster) {
	s.say("You slay %s!", m.Name)
	s.Kills++
	s.gainXP(m.MaxHP)
	s.P.GitEnergy = min(100, s.P.GitEnergy+15)
	if m.Boss {
		s.say("%s is defeated! You win!", m.Name)
		s.Outcome = Won
	}
}

// attack makes the player hit m. A kill gives XP equal to the monster's MaxHP.
// Slaying the boss ends the run as Won. The slain monster is not removed from
// the slice: its HP stays <= 0. That keeps monster indices stable and the
// monster phase deterministic (SPEC 7.3).
func (s *State) attack(m *dungeon.Monster) {
	atk := s.P.ATK
	if s.P.Weapon != nil {
		atk += s.P.Weapon.BonusATK
	}
	dmg := s.roll(atk)
	if s.P.Weapon != nil && s.P.Weapon.Archetype == dungeon.Dagger {
		dmg *= s.daggerCritical(m)
	}
	m.HP -= dmg
	s.say("You hit %s for %d.", m.Name, dmg)
	if m.HP <= 0 {
		s.slay(m)
		return
	}
	if s.P.Weapon != nil && s.P.Weapon.Archetype == dungeon.Hammer {
		d := dungeon.Point{X: sign(m.Pos.X - s.P.Pos.X), Y: sign(m.Pos.Y - s.P.Pos.Y)}
		s.hammerKnockback(m, d)
	}
}

// gainXP adds n XP and levels up while XP >= 20*Level. It is a loop because
// one big kill can pass several thresholds. Each level gives +5 MaxHP,
// up to +5 HP (capped at MaxHP) and +1 ATK.
func (s *State) gainXP(n int) {
	p := &s.P
	p.XP += n
	for p.XP >= 20*p.Level {
		p.XP -= 20 * p.Level
		p.Level++
		p.MaxHP += 5
		p.HP = min(p.MaxHP, p.HP+5)
		p.ATK++
		s.say("You feel stronger! Level %d.", p.Level)
	}
}

// sign returns -1, 0 or 1 for the sign of n.
func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

// abs returns the absolute value of n.
func abs(n int) int { return max(n, -n) }

// monsterPhase lets each awake, living monster act once, in slice order (SPEC 7.3).
// A monster next to the player (Manhattan distance 1) attacks. If that kills
// the player the phase ends at once. Otherwise the monster takes one step
// toward the player along the longer axis first, then the shorter one.
// A step is valid if the tile is walkable and holds neither the player nor
// another living monster. Monsters do not path around obstacles; they stay put.
// Diagonal neighbours are not adjacent, so they step instead of attacking.
func (s *State) monsterPhase() {
	s.P.LastDamageTaken = 0
	l := s.level()
	for i := range l.Monsters {
		m := &l.Monsters[i]
		if m.HP <= 0 || !m.Awake {
			continue
		}
		if m.StunTurns > 0 {
			m.StunTurns--
			if m.Telegraph {
				m.Telegraph = false
			}
			continue
		}
		if s.P.StashTurns > 0 {
			if m.Telegraph {
				m.Telegraph = false
				s.say("%s's heavy blow misses!", m.Name)
			}
			continue
		}
		dx, dy := s.P.Pos.X-m.Pos.X, s.P.Pos.Y-m.Pos.Y
		if m.Telegraph {
			m.Telegraph = false
			if abs(dx)+abs(dy) == 1 {
				dmg := (s.roll(m.ATK) * 3) / 2
				s.P.HP -= dmg
				s.P.LastDamageTaken += dmg
				s.say("%s hits you for %d.", m.Name, dmg)
				if s.P.HP <= 0 {
					s.P.HP = 0
					s.Outcome = Died
					s.KilledBy = m.Name
					s.say("You die...")
					return
				}
			} else {
				s.say("%s's heavy blow misses!", m.Name)
			}
			continue
		}
		if abs(dx)+abs(dy) == 1 {
			if s.rng.IntN(3) == 0 {
				m.Telegraph = true
				s.say("%s winds up a heavy blow!", m.Name)
				continue
			}
			dmg := s.roll(m.ATK)
			s.P.HP -= dmg
			s.P.LastDamageTaken += dmg
			s.say("%s hits you for %d.", m.Name, dmg)
			if s.P.HP <= 0 {
				s.P.HP = 0
				s.Outcome = Died
				s.KilledBy = m.Name
				s.say("You die...")
				return
			}
			continue
		}
		tries := [2]dungeon.Point{{X: sign(dx)}, {Y: sign(dy)}}
		if abs(dx) < abs(dy) {
			tries[0], tries[1] = tries[1], tries[0]
		}
		for _, d := range tries {
			t := dungeon.Point{X: m.Pos.X + d.X, Y: m.Pos.Y + d.Y}
			if d != (dungeon.Point{}) && dungeon.Walkable(l.At(t)) && t != s.P.Pos && s.monsterAt(t) == nil {
				m.Pos = t
				break
			}
		}
	}
}

// blameTarget finds the target monster for git blame, prioritizing the player's
// facing/last move direction, then the adjacent monster with the highest HP.
func (s *State) blameTarget() *dungeon.Monster {
	if s.lastDir != (dungeon.Point{}) {
		facingPos := dungeon.Point{X: s.P.Pos.X + s.lastDir.X, Y: s.P.Pos.Y + s.lastDir.Y}
		if m := s.monsterAt(facingPos); m != nil && m.HP > 0 {
			return m
		}
	}
	var best *dungeon.Monster
	for _, d := range []dungeon.Point{{X: 0, Y: -1}, {X: 0, Y: 1}, {X: -1, Y: 0}, {X: 1, Y: 0}} {
		adjPos := dungeon.Point{X: s.P.Pos.X + d.X, Y: s.P.Pos.Y + d.Y}
		if m := s.monsterAt(adjPos); m != nil && m.HP > 0 {
			if best == nil || m.HP > best.HP {
				best = m
			}
		}
	}
	return best
}

// castGitPower executes a tactical Git Power mapped to hotkeys 1, 2, or 3.
// Returns true if the power was successfully cast and consumed a turn.
func (s *State) castGitPower(key string) bool {
	switch key {
	case "1": // git stash (Cost 30)
		if s.P.GitEnergy < 30 {
			s.say("Not enough Git Energy.")
			return false
		}
		s.P.GitEnergy -= 30
		s.P.StashTurns = 2
		s.say("You git stash into the shadows.")
		return true

	case "2": // git revert (Cost 25)
		if s.P.LastDamageTaken == 0 {
			s.say("No damage to revert.")
			return false
		}
		if s.P.GitEnergy < 25 {
			s.say("Not enough Git Energy.")
			return false
		}
		heal := min(s.P.LastDamageTaken, s.P.MaxHP-s.P.HP)
		s.P.HP += heal
		s.P.LastDamageTaken = 0
		s.P.GitEnergy -= 25
		s.say("You revert the damage! +%d HP.", heal)
		return true

	case "3": // git blame (Cost 20)
		target := s.blameTarget()
		if target == nil {
			s.say("No adjacent enemy to blame.")
			return false
		}
		if s.P.GitEnergy < 20 {
			s.say("Not enough Git Energy.")
			return false
		}
		s.P.GitEnergy -= 20
		target.StunTurns = 2
		target.ATK = max(1, target.ATK/2)
		target.Telegraph = false
		s.say("You blame %s! It is stunned.", target.Name)
		return true
	}
	return false
}
