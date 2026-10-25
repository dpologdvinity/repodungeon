# Combat, Weapons, and Git Powers Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform Repodungeon from a pure bump-attack attrition death trap into a tactical, winnable git roguelike with collectible weapons, resting recovery, monster attack telegraphs, and active git powers.

**Architecture:** Extend core dungeon generation with weapon items derived from repository build/tool files, augment the pure game state machine with weapon attack archetypes (spear reach, hammer knockback, dagger criticals), natural resting health recovery, telegraphed monster attacks, and an energy-fueled Git Powers ability system (`stash`, `revert`, `blame`), then update 80x24 TUI rendering and color formatting.

**Tech Stack:** Go 1.26+, Bubble Tea v2 (`charm.land/bubbletea/v2`), stdlib `testing`, SplitMix64 RNG.

**Spec:** `docs/specs/2026-10-25-combat-and-weapons-overhaul-design.md`

## Global Constraints

- Go 1.26.0+ in `go.mod`.
- Fixed 80x24 screen resolution in `Render()`.
- Deterministic SplitMix64 RNG; all generation uses `NewRNG(seed)`, all combat rolls use `NewRNG(seed + 1)`.
- No new external dependencies beyond Bubble Tea v2.
- Git operations remain strictly read-only.
- All game logic remains pure with zero terminal, clock, or I/O dependencies.

## Review Focus

1. **Spear targeting dead or obstructed entities:** Moving toward a dead monster or an obstacle at 1-tile distance must not trigger a phantom spear attack through walls or target corpse entities.
2. **`git revert` with zero damage taken or on turn 0:** Using `git revert` when no damage has been received in the previous monster phase must safely consume no energy and inform the player that no damage was taken.
3. **Rest healing at MaxHP:** Pressing `.` when already at `P.MaxHP` must not heal past `MaxHP` or flood the message log.
4. **`git blame` with no adjacent targets:** Pressing `3` when no living monster is adjacent must fail gracefully with a message and consume zero energy.
5. **Weapon pickup with full pack:** Picking up a weapon with a full pack (10 items) when already holding an equipped weapon must leave the floor weapon in place and display `Your pack is full.`

---

### Task 1: Dungeon Weapon Item Types and Generation

**Files:**
- Modify: `internal/dungeon/gen.go:102-120`
- Test: `internal/dungeon/gen_test.go`

**Interfaces:**
- Produces:
  - `ItemKind` enum with `Weapon ItemKind = 2`
  - `WeaponArchetype uint8`: `Sword`, `Spear`, `Hammer`, `Dagger`
  - `Item` struct extended with `Archetype WeaponArchetype`, `BonusATK int`
  - `(k ItemKind) Glyph() rune`: returns `')'` for `Weapon`
  - `(k ItemKind) Name() string`: returns weapon name or archetype description
- Consumes:
  - `repo.File`, `Room`, `Level`

- [ ] **Step 1: Write the failing test for weapon item generation**

In `internal/dungeon/gen_test.go`:
```go
func TestWeaponSpawning(t *testing.T) {
    // Verify that tool files (e.g. Makefile, Dockerfile, go.mod) generate Weapon items with glyph ')'
    files := []repo.File{
        {Path: "Makefile", Lines: 30, Churn: 5},
        {Path: "main.go", Lines: 100, Churn: 10},
    }
    r := &repo.Repo{Name: "test", Head: "0123456789abcdef", Files: files}
    d, err := Generate(r)
    if err != nil {
        t.Fatalf("Generate failed: %v", err)
    }
    hasWeapon := false
    for _, it := range d.Levels[0].Items {
        if it.Kind == Weapon {
            hasWeapon = true
            if it.Kind.Glyph() != ')' {
                t.Errorf("expected weapon glyph ')', got %c", it.Kind.Glyph())
            }
        }
    }
    if !hasWeapon {
        t.Errorf("expected at least one weapon to spawn")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dungeon -run TestWeaponSpawning`  
Expected: FAIL with undefined `Weapon`

- [ ] **Step 3: Implement weapon types and spawning in `internal/dungeon/gen.go`**

Define `Weapon ItemKind = 2`, `WeaponArchetype` constants (`Sword`, `Spear`, `Hammer`, `Dagger`), and extend `Item` fields. Update `buildLevel` to spawn weapon items for tool files (`Makefile`, `Dockerfile`, `go.mod`, scripts) and ensure at least 1 weapon spawns per level.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/dungeon -run TestWeaponSpawning`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/dungeon/gen.go internal/dungeon/gen_test.go
git commit -m "feat(dungeon): add weapon items and generator spawning"
```

---

### Task 2: Player Weapon Inventory and Equipping

**Files:**
- Modify: `internal/game/game.go:49-58`, `internal/game/game.go:222-235`
- Test: `internal/game/game_test.go`

**Interfaces:**
- Produces:
  - `Player.Weapon *dungeon.Item` (nil if unarmed)
  - `s.equipWeapon(i int)`
- Consumes:
  - `dungeon.Weapon`, `dungeon.Item`

- [ ] **Step 1: Write the failing test for equipping weapons and full-pack pickup edge case**

In `internal/game/game_test.go`:
```go
func TestWeaponEquipAndPackLimit(t *testing.T) {
    d := dungeon.FromArt([]string{
        "---|",
        "|@.|",
        "---|",
    })
    s := New(d)
    w := dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Sword, BonusATK: 3}
    s.P.Inv = append(s.P.Inv, w)
    
    // Test equipping weapon from inventory slot 0
    s.inventoryKey("a")
    if s.P.Weapon == nil || s.P.Weapon.BonusATK != 3 {
        t.Errorf("expected weapon equipped with +3 ATK")
    }
    
    // Review Focus #5: Full pack with equipped weapon
    for i := 0; i < packSize; i++ {
        s.P.Inv = append(s.P.Inv, dungeon.Item{Kind: dungeon.Potion})
    }
    s.level().Items = append(s.level().Items, dungeon.Item{Kind: dungeon.Weapon, Pos: dungeon.Point{X: 2, Y: 1}})
    s.Apply("l") // step onto weapon
    if len(s.P.Inv) > packSize {
        t.Errorf("pack overflowed beyond %d items", packSize)
    }
    if s.Msgs[len(s.Msgs)-1] != "Your pack is full." {
        t.Errorf("expected full pack message, got %v", s.Msgs)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game -run TestWeaponEquipAndPackLimit`  
Expected: FAIL (`P.Weapon` undefined)

- [ ] **Step 3: Implement weapon equipping and inventory usage in `internal/game/game.go`**

Add `Weapon *dungeon.Item` to `Player`. In `use(i int)`: if `it.Kind == dungeon.Weapon`, equip the weapon, placing the previously equipped weapon (if any) into index `i` of `P.Inv`. If player is unarmed, picking up a weapon on the floor auto-equips it if inventory is empty.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game -run TestWeaponEquipAndPackLimit`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/game.go internal/game/game_test.go
git commit -m "feat(game): add weapon equipping and inventory swapping"
```

---

### Task 3: Weapon Combat Mechanics (Spear Reach, Hammer Knockback, Dagger Critical)

**Files:**
- Modify: `internal/game/game.go:292-334`
- Test: `internal/game/game_test.go`

**Interfaces:**
- Produces:
  - `spearReachAttack(d Point) bool`
  - `hammerKnockback(m *dungeon.Monster, d Point)`
  - `daggerCritical(m *dungeon.Monster) int`
- Consumes:
  - `Player.Weapon`, `dungeon.WeaponArchetype`

- [ ] **Step 1: Write failing tests for weapon mechanics and Review Focus #1 (Spear targeting dead or obstructed entities)**

In `internal/game/game_test.go`:
```go
func TestWeaponCombatMechanics(t *testing.T) {
    // 1. Spear 2-tile reach attack
    d := dungeon.FromArt([]string{
        "------|",
        "|@..m.|",
        "------|",
    })
    s := New(d)
    s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Spear, BonusATK: 2}
    s.level().Monsters = append(s.level().Monsters, dungeon.Monster{
        Name: "target.go", Pos: dungeon.Point{X: 3, Y: 1}, HP: 10, MaxHP: 10, ATK: 2,
    })
    
    // Pressing 'l' (right) toward monster at distance 2 should attack without changing P.Pos
    startPos := s.P.Pos
    s.Apply("l")
    if s.P.Pos != startPos {
        t.Errorf("spear attack should not advance player position: %v vs %v", s.P.Pos, startPos)
    }
    if s.level().Monsters[0].HP >= 10 {
        t.Errorf("spear attack should have damaged monster, HP is %d", s.level().Monsters[0].HP)
    }

    // Review Focus #1: Spear obstructed by wall should not attack through walls
    s.level().Set(dungeon.Point{X: 2, Y: 1}, dungeon.WallV)
    hpBefore := s.level().Monsters[0].HP
    s.Apply("l")
    if s.level().Monsters[0].HP != hpBefore {
        t.Errorf("spear should not attack through solid wall")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game -run TestWeaponCombatMechanics`  
Expected: FAIL

- [ ] **Step 3: Implement weapon mechanics in `internal/game/game.go`**

Implement:
1. Spear: in `move(d)`: if equipped weapon is `Spear`, check 1-tile and 2-tile points. If tile 1 is walkable and tile 2 has a living monster, perform reach attack without moving player.
2. Hammer: on hit, push target monster 1 tile along vector `d`. If target tile is blocked, deal +2 collision damage.
3. Dagger: if target monster is `!Awake` or player in `git stash` stealth, double the rolled damage.
4. Add weapon `BonusATK` to damage rolls.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game -run TestWeaponCombatMechanics`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/game.go internal/game/game_test.go
git commit -m "feat(game): implement spear reach, hammer knockback, and dagger criticals"
```

---

### Task 4: Resting and Natural Healing

**Files:**
- Modify: `internal/game/game.go:190-205`
- Test: `internal/game/game_test.go`

**Interfaces:**
- Produces:
  - `s.safeRestTurn()`
  - `s.hasAwakeMonstersInFOV() bool`
- Consumes:
  - `s.P.HP`, `s.P.MaxHP`

- [ ] **Step 1: Write failing test for resting and Review Focus #3 (resting at MaxHP)**

In `internal/game/game_test.go`:
```go
func TestRestingAndMaxHPConstraint(t *testing.T) {
    d := dungeon.FromArt([]string{
        "----|",
        "|@..|",
        "----|",
    })
    s := New(d)
    s.P.HP = 20
    s.P.MaxHP = 30
    
    // 4 safe wait turns should restore 1 HP
    for i := 0; i < 4; i++ {
        s.Apply(".")
    }
    if s.P.HP != 21 {
        t.Errorf("expected HP to heal to 21 after 4 safe waits, got %d", s.P.HP)
    }

    // Review Focus #3: resting at MaxHP must not overflow or send spurious messages
    s.P.HP = s.P.MaxHP
    s.Msgs = nil
    for i := 0; i < 4; i++ {
        s.Apply(".")
    }
    if s.P.HP > s.P.MaxHP {
        t.Errorf("HP overflowed MaxHP: %d > %d", s.P.HP, s.P.MaxHP)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game -run TestRestingAndMaxHPConstraint`  
Expected: FAIL

- [ ] **Step 3: Implement resting in `internal/game/game.go`**

Add `restWaitCount int` to `State`. In `playKey(".")`: check `hasAwakeMonstersInFOV()`. If false and `s.P.HP < s.P.MaxHP`, increment `restWaitCount`. Every 4 turns: `s.P.HP++`, say `You rest quietly. +1 HP.`. If awake monsters exist, reset `restWaitCount = 0`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game -run TestRestingAndMaxHPConstraint`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/game.go internal/game/game_test.go
git commit -m "feat(game): add natural health regeneration on safe wait"
```

---

### Task 5: Monster Attack Telegraphing

**Files:**
- Modify: `internal/dungeon/gen.go:85-100`, `internal/game/game.go:365-405`
- Test: `internal/game/game_test.go`

**Interfaces:**
- Produces:
  - `Monster.Telegraph bool`
  - Telegraphed heavy attack with 1.5x damage on hit, miss on dodge
- Consumes:
  - `monsterPhase()`

- [ ] **Step 1: Write failing test for monster attack telegraphing and dodging**

In `internal/game/game_test.go`:
```go
func TestMonsterTelegraphAndDodge(t *testing.T) {
    d := dungeon.FromArt([]string{
        "------|",
        "|@m...|",
        "------|",
    })
    s := New(d)
    m := &s.level().Monsters[0]
    m.Awake = true
    m.Telegraph = true // Monster is telegraphing heavy strike
    
    // Player steps away ('k' or right if space, or moves left/waits)
    // If player stays adjacent: takes 1.5x damage
    // If player steps away to distance > 1: attack misses!
    s.P.Pos = dungeon.Point{X: 1, Y: 1}
    m.Pos = dungeon.Point{X: 2, Y: 1}
    hpBefore := s.P.HP
    s.Apply("k") // bump against wall / stays adjacent -> takes telegraphed hit
    if s.P.HP >= hpBefore {
        t.Errorf("expected damage from telegraphed strike")
    }
    if m.Telegraph {
        t.Errorf("telegraph flag should reset after striking")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game -run TestMonsterTelegraphAndDodge`  
Expected: FAIL (`Telegraph` undefined)

- [ ] **Step 3: Implement monster telegraphing in `internal/dungeon/gen.go` and `internal/game/game.go`**

Add `Telegraph bool` to `Monster`. In `monsterPhase()`: when adjacent to player, if not telegraphed, 33% chance to telegraph heavy blow instead of normal hit (says `<name> winds up a heavy blow!`). On next turn, if still adjacent, deals `1.5 * dmg`; if player moved away, attack misses (says `<name>'s heavy blow misses!`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game -run TestMonsterTelegraphAndDodge`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/dungeon/gen.go internal/game/game.go internal/game/game_test.go
git commit -m "feat(game): add monster attack telegraphs and evasion"
```

---

### Task 6: Git Powers System (Energy, Stash, Revert, Blame)

**Files:**
- Modify: `internal/game/game.go:49-88`, `internal/game/game.go:190-220`
- Test: `internal/game/game_test.go`

**Interfaces:**
- Produces:
  - `Player.GitEnergy int`
  - `Player.StashTurns int`
  - `Player.LastDamageTaken int`
  - `s.castGitPower(key string) bool`
- Consumes:
  - Hotkeys `"1"`, `"2"`, `"3"`

- [ ] **Step 1: Write failing tests for Git Powers and Review Focus #2, #4**

In `internal/game/game_test.go`:
```go
func TestGitPowersAndFocusChecks(t *testing.T) {
    d := dungeon.FromArt([]string{
        "------|",
        "|@.m..|",
        "------|",
    })
    s := New(d)
    s.P.GitEnergy = 100

    // Review Focus #2: 'git revert' (key 2) with 0 damage taken
    s.P.LastDamageTaken = 0
    s.Apply("2")
    if s.P.GitEnergy != 100 {
        t.Errorf("git revert should not consume energy when no damage was taken")
    }

    // Review Focus #4: 'git blame' (key 3) with no adjacent monster
    s.P.Pos = dungeon.Point{X: 1, Y: 1}
    s.level().Monsters[0].Pos = dungeon.Point{X: 5, Y: 1}
    s.Apply("3")
    if s.P.GitEnergy != 100 {
        t.Errorf("git blame should not consume energy when no enemy is adjacent")
    }

    // Successful git stash (key 1)
    s.Apply("1")
    if s.P.GitEnergy != 70 { // cost 30
        t.Errorf("expected energy 70 after stash, got %d", s.P.GitEnergy)
    }
    if s.P.StashTurns != 2 {
        t.Errorf("expected 2 turns of stash stealth")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game -run TestGitPowersAndFocusChecks`  
Expected: FAIL

- [ ] **Step 3: Implement Git Powers in `internal/game/game.go`**

Add `GitEnergy`, `StashTurns`, and `LastDamageTaken` to `Player`.
Handle keys `"1"`, `"2"`, `"3"`:
- `"1"`: `git stash` (Cost 30). In `monsterPhase()`, awake monsters do not attack or pursue while `StashTurns > 0`.
- `"2"`: `git revert` (Cost 25). If `LastDamageTaken > 0`, heal `min(LastDamageTaken, MaxHP - HP)` and reset `LastDamageTaken = 0`.
- `"3"`: `git blame` (Cost 20). If adjacent monster exists, stun it for 2 turns and halve its ATK.
Charge energy: +1 on newly seen floor tile in `updateFOV()`, +15 on monster slay in `attack()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game -run TestGitPowersAndFocusChecks`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/game.go internal/game/game_test.go
git commit -m "feat(game): add Git Powers stash, revert, and blame"
```

---

### Task 7: 80x24 TUI Rendering, Colors, and Help Screen

**Files:**
- Modify: `internal/game/render.go:1-214`
- Modify: `internal/tui/color.go:1-120`
- Test: `internal/game/render_test.go`, `internal/tui/color_test.go`

**Interfaces:**
- Produces:
  - 80x24 layout with Energy and equipped weapon in status rows (Row 22 & 23)
  - Colorized weapon glyph `)` and telegraphed monster indicator
- Consumes:
  - `Player.Weapon`, `Player.GitEnergy`, `Monster.Telegraph`

- [ ] **Step 1: Write failing test for 80x24 rendering with new status elements**

In `internal/game/render_test.go`:
```go
func TestRenderNewSystems(t *testing.T) {
    d := dungeon.FromArt([]string{
        "---|",
        "|@.|",
        "---|",
    })
    s := New(d)
    s.P.Weapon = &dungeon.Item{Kind: dungeon.Weapon, Archetype: dungeon.Spear, BonusATK: 2}
    s.P.GitEnergy = 75
    
    out := s.Render()
    lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
    if len(lines) != 24 {
        t.Fatalf("expected exactly 24 screen rows, got %d", len(lines))
    }
    for i, line := range lines {
        if len([]rune(line)) > 80 {
            t.Errorf("line %d exceeds 80 runes: %q", i, line)
        }
    }
    if !strings.Contains(lines[22], "EN 75") && !strings.Contains(lines[22], "Spear") {
        t.Errorf("status line missing energy or weapon info: %q", lines[22])
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game -run TestRenderNewSystems`  
Expected: FAIL

- [ ] **Step 3: Update `internal/game/render.go` and `internal/tui/color.go`**

Update `cell()` to draw `)` for weapons and alert markers for telegraphed monsters. Update `status()` on row 22 to fit within 80 runes: `Depth <d>  HP <h>/<m>  ATK <a>  EN <e>  [Wpn: <w>]  LV <l>`. Update `helpRows()` to explain weapons, rest, and git powers. Add ANSI color formatting for `)` (blue) and telegraphed monsters (bold red).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game -run TestRenderNewSystems && go test ./internal/tui`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/render.go internal/tui/color.go internal/game/render_test.go internal/tui/color_test.go
git commit -m "feat(ui): update 80x24 screen, status HUD, and ANSI colors"
```

---

### Task 8: End-to-End Verification & Golden Files

**Files:**
- Modify: `internal/dungeon/testdata/golden/*`, `internal/golden/*`
- Test: All package tests (`./...`)

**Interfaces:**
- Produces:
  - Clean test suite passing across all packages (`cli`, `dungeon`, `game`, `golden`, `repo`, `testrepo`, `tui`).

- [ ] **Step 1: Run complete test suite and update goldens**

Run: `go test -update ./...`  
Run: `go test -v ./...`  
Expected: All tests pass.

- [ ] **Step 2: Verify `gofmt` and `go vet`**

Run: `gofmt -l .`  
Expected: empty output  
Run: `go vet ./...`  
Expected: clean output

- [ ] **Step 3: Commit**

```bash
git add internal/dungeon/testdata/ internal/golden/testdata/
git commit -m "test: update golden test data for combat overhaul"
```
