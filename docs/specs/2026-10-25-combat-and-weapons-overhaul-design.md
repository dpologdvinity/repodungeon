# Design Document: Combat, Weapons, and Git Powers Overhaul

**Date:** 2026-10-25  
**Topic:** Combat, Weapons, and Git Powers Overhaul  
**Status:** In Review  

---

## 1. Intent and Why

In v0.1, Repodungeon is a pure attrition death trap. The player starts with 30 HP and 3 ATK. Mid-size file monsters have 15–25 HP and hit for ~3 damage per turn, costing 15–20 HP in a single bump fight. With no natural healing, zero tactical defenses, and potions only spawning if repository source files contain literal `TODO` comments, runs are frequently unwinnable and lack meaningful player agency.

This overhaul transforms Repodungeon into an engaging, tactical git-themed roguelike where skill, positioning, weapon choices, and git abilities allow players to survive and win.

---

## 2. Goals, Non-Goals, and Anti-Goals

### Goals
1. **Weapon System**:
   - Introduce weapons as collectible items (tile glyph `)`) on dungeon floors and as drops from defeated tool/build files (`Makefile`, `Dockerfile`, `go.mod`, scripts).
   - Equipping weapons alters combat mechanics (e.g. spears grant 2-tile reach, hammers apply knockback, daggers grant high critical damage to unalert foes, swords offer reliable bonus damage).
   - Player can view and equip weapons from the inventory menu.
2. **Rest and Natural Healing**:
   - Waiting (`.`) in a room with no awake monsters gradually restores player HP (e.g. 1 HP every 4 turns).
   - Taking damage or waking an enemy interrupts rest.
3. **Tactical Combat & Enemy Telegraphs**:
   - Awake monsters telegraph heavy strikes one turn in advance (displaying a warning indicator such as `!` or flashing).
   - Moving away or behind a corner dodges the telegraphed hit.
   - Doorways and corridors serve as intentional choke points against swarms.
4. **Git Powers (Active Abilities)**:
   - Provide active tactical powers mapped to keys `1`–`4`:
     - `1: git stash`: Phase out / become invulnerable for 2 turns to reposition or escape swarms.
     - `2: git revert`: Undo the previous monster strike, healing back the damage taken.
     - `3: git blame`: Stun target adjacent monster for 2 turns and lower its defense.
   - Powers consume Git Energy, charged by exploring new rooms and defeating file monsters.
5. **Preserved Determinism**:
   - All new combat actions, RNG rolls, and item placements remain 100% deterministic based on the commit seed.

### Non-Goals
- Adding diagonal 8-way movement (movement remains cardinal 4-way via `hjkl` and arrows).
- Adding full raycasting line-of-sight algorithms (keep fast Manhattan/circular bounds).
- Cross-run meta-progression or persistent save files (runs remain self-contained roguelike runs).

### Anti-Goals
- Trivializing combat so the player can mindlessly bump without thinking.
- Overcrowding the fixed 80x24 terminal display with cluttered menus.
- Breaking existing CLI flags, read-only git safety, or the post-run summary card.

---

## 3. Constraints

- **Fixed Screen Size**: 80 columns by 24 rows (`Render()` string format).
- **Go Version & Dependencies**: Go 1.26+, Bubble Tea v2 (`charm.land/bubbletea/v2`), no new external dependencies.
- **Git Read-Only**: Only non-mutating git inspection commands (`cat-file`, `ls-tree`, `log`, `rev-parse`).
- **Determinism**: Pure state transitions in `internal/game` driven by string key presses, reproducible via `rng.go` SplitMix64.

---

## 4. Systems Design

### 4.1 Weapon System
- **Item Kind**: Extend `dungeon.ItemKind` to include `Weapon`.
- **Weapon Archetypes**:
  - `Sword`: Standard melee, +2 to +4 ATK.
  - `Spear`: 2-tile linear reach; attacks target enemy up to 2 tiles away without stepping into their melee range.
  - `Hammer`: Heavy blunt damage (+3 ATK), knocks target back 1 tile on hit. If target is blocked by wall/monster, deals +2 bonus collision damage.
  - `Dagger`: Fast strikes; deals double damage against unalert enemies or when striking an enemy from the side/back.
- **Spawning Rules**:
  - Repository files associated with tooling or builds (`Makefile`, `Dockerfile`, `*.sh`, `go.mod`, `package.json`, CI configs) spawn a weapon item in their room.
  - If a level has no tool files, at least 1 weapon is guaranteed to spawn per depth floor.
- **Inventory & Equipping**:
  - Picking up a weapon places it in the pack (pack size stays 10).
  - Opening inventory (`i`) displays held weapons with their weapon archetype and bonus stats.
  - Selecting a weapon equips it into the active weapon slot (`P.Weapon`). The previously equipped weapon returns to the pack.

### 4.2 Rest and Recovery
- When pressing `.` (wait) in Play mode:
  - If any living monster in the current room (or within FOV) is `Awake`, normal 1-turn wait occurs.
  - If no awake monsters are in sight, wait regenerates 1 HP every 4 wait turns up to `P.MaxHP`.
  - Monsters walking into FOV immediately interrupt rest and wake up.

### 4.3 Monster Telegraphing
- Awake monsters alternate between standard behaviors and telegraphed strikes:
  - When preparing a heavy blow, monster state sets `Telegraph = true`.
  - During rendering, telegraphed monsters are drawn with an alert marker (or `!` glyph).
  - On the following turn, if player is still adjacent, the telegraphed strike deals 1.5x damage. If the player stepped away, the attack misses completely and `Telegraph` clears.

### 4.4 Git Powers
- The player has an Energy pool (`GitEnergy`, max 100, starts at 50).
- Gaining Energy:
  - Exploring a previously unseen room floor tile: +1 Energy.
  - Slaying a file monster: +15 Energy.
- Abilities:
  1. `git stash` (Cost: 30 Energy, Key `1`): Player enters Stash state for 2 turns. Monsters cannot attack or target the player; player can reposition freely.
  2. `git revert` (Cost: 25 Energy, Key `2`): Refunds the damage received on the prior monster phase.
  3. `git blame` (Cost: 20 Energy, Key `3`): Target adjacent monster is stunned for 2 turns and has its ATK halved for the rest of combat.
- UI: Row 22 of the screen is updated to display `Energy <cur>/<max>` and available ability hotkeys `[1:Stash 2:Revert 3:Blame]`.

---

## 5. What is Left to the Builder

- Exact numeric balancing for monster scaling vs weapon damage formulas.
- Precise HUD string formatting in Row 22 and Row 23 to ensure line length does not exceed 80 runes.
- ANSI color styling in `internal/tui/color.go` for weapon items (e.g. bold blue `)`) and telegraph markers (flashing or bright red).
- Unit and golden test cases covering weapon attacks, reach calculations, knockbacks, energy accumulation, and git power execution.

---

## 6. Resolved Builder Calls

1. **Spear Attack vs Movement**:
   - When equipped with a spear, pressing a direction toward an enemy 2 tiles away (where the intervening tile is unobstructed) performs a 2-tile reach attack without advancing the player's position.
   - Spears cannot reach through walls or closed doors.
2. **Dagger Critical Condition**:
   - Dagger double damage triggers when striking an enemy that is currently sleeping (`!m.Awake`), or when striking from `git stash` stealth.
3. **Turn-Based Wait & Rest**:
   - Each `.` press consumes 1 turn. If no awake enemies are within view, every 4 turns of waiting restores 1 HP.
4. **Git Power Targeting & Resolution**:
   - `git blame` (key `3`): Hits an adjacent monster (prioritizes facing direction, then highest HP) without requiring a second targeting prompt.
   - `git revert` (key `2`): Refunds damage taken in the last enemy turn while the player is still alive. HP <= 0 remains permadeath.

