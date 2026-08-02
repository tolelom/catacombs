# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Catacombs is a multiplayer roguelike dungeon crawler with dual access: SSH (native TUI) and HTTP/WebSocket (web browser via xterm.js). Written in Go, it uses Bubble Tea for the terminal UI and BoltDB for persistence.

## Build & Run Commands

```bash
go build -o catacombs .          # Build
go test ./...                     # Run all tests
go test ./combat/                 # Run tests for a single package
go test ./entity/ -run TestName   # Run a specific test
go vet ./...                      # Lint
```

Docker (local):
```bash
docker build -t catacombs .
docker-compose up                 # SSH on :2222, HTTP on :8080
```

## Deployment (tolelom.xyz)

The game runs as a Docker container on the Mac Mini server, behind Caddy reverse proxy.

**Server structure:**
```
~/server/
├── docker-compose.yml      # All services (caddy, gitea, catacombs, etc.)
├── caddy/Caddyfile          # Reverse proxy config
└── apps/catacombs/          # Git clone of this repo
```

**First-time setup:**
```bash
cd ~/server
git clone https://git.tolelom.xyz/tolelom/Catacombs.git apps/catacombs
```

Add to `docker-compose.yml` services:
```yaml
  catacombs:
    build: ./apps/catacombs
    restart: always
    logging: *default-logging
    volumes:
      - catacombs_data:/app/data
    networks:
      - web
    deploy:
      resources:
        limits:
          memory: 64m
          pids: 200
```

Add `catacombs_data:` to the `volumes:` section.

Add to `caddy/Caddyfile`:
```
catacombs.tolelom.xyz {
    encode gzip zstd
    reverse_proxy catacombs:8080
}
```

**Deploy / Update:**
```bash
cd ~/server/apps/catacombs && git pull
cd ~/server && docker compose up -d --build catacombs
docker restart server-caddy-1    # only needed if Caddyfile changed
```

**Access:** https://catacombs.tolelom.xyz/static/

## Architecture

**Package dependency flow:** `main` → `server`/`web`/`store` → `game` → `dungeon` → `entity` → `combat`

| Package | Responsibility |
|---------|---------------|
| `main.go` | Entry point: initializes BoltDB (`./data/catacombs.db`), starts SSH server (:2222) and HTTP server (:8080), runs the backup scheduler, calls `Lobby.StopAllSessions` on graceful shutdown |
| `game/` | `Lobby` (room management, player tracking, reconnection, `EnsureSession` / `LeaveSession` / `StopAllSessions`), `GameSession` (turn-based state, per-session deterministic `*rand.Rand`), turn execution with `cfg.Game.TurnTimeoutSec` action timeout, room events (combat/shop/treasure/event), weekly `Mutation` consulted at runtime via `effectiveSkillUses` / `effectiveTurnTimeout` (no shared-config mutation) |
| `ui/` | Bubble Tea state machine. Screens (each in its own file): `title` → `nickname` → `lobby` → `class_view` → `waiting` → `game_view` → `shop_view` → `result_view`, plus `help`, `stats`, `leaderboard`, `achievements`, `codex`. `model.go` is the thin top-level dispatcher (~170 lines). `Context` carries shared deps (Lobby, Store, Session, Fingerprint, PlayerName, RoomCode, HardMode) |
| `dungeon/` | BSP tree procedural generation (60×20 maps), ASCII rendering with floor themes, room visibility / corridor flood-fill. `GenerateFloor` accepts the session RNG for daily-challenge determinism |
| `entity/` | `Player` (4 classes: Warrior/Mage/Healer/Rogue), `Monster` (4 regular + 4 mini-bosses + 4 bosses; `BaseName()` strips elite prefixes for codex), Items, Relics, `PlayerSkills` diamond trees (2 branches per class; T1 auto → T2 fork → T3 auto → T4 fork) |
| `combat/` | `CalcDamage`, `MonsterAI`, `ResolveAttacks`, `AttemptFlee`, `Combos`. All RNG-consuming entry points take a leading `*rand.Rand` (nil falls back to a process-global) so daily runs stay deterministic |
| `store/` | BoltDB persistence: profiles, rankings, achievements (10 IDs), unlocks, titles, codex, daily records, passwords. `Backup` snapshots and `PruneBackups(dir, keep)` rotates them |
| `server/` | Wish SSH server. Public-key clients hash to `SHA256:...` fingerprint; the web bridge has no key, so the middleware mints a per-connection `anon:<random hex>` so lobby bookkeeping doesn't collide |
| `web/` | HTTP + WebSocket bridge to local SSH (`/ws`), embedded xterm.js frontend, `/admin` JSON stats endpoint with constant-time Basic Auth |

## Key Patterns

- **Concurrent session management**: `GameSession.mu` protects all state; UI snapshots via `GetState()` (deep copy). `Lobby.mu` separately protects the room map; never call into a session under the lobby lock
- **Turn-based action collection**: `cfg.Game.TurnTimeoutSec` window; absent submissions default to `ActionWait`; out (dead/fled) players are skipped. `RunTurn` drains stale actions before the new collection window
- **Per-player leave**: `Lobby.LeaveSession(code, fingerprint)` removes one player from the session; the room and combat loop only tear down when the last player leaves
- **Deterministic daily**: `GameSession.rng` is reseeded from `DailySeed(date) + floor` in DailyMode. Don't introduce calls to package-level `rand` in content paths — route through `s.rng`
- **Reconnect by fingerprint**: `Lobby.activeSessions` maps fingerprint → room code. `Title.Update` checks it on Enter; reconnects land directly in the running session
- **Dual access**: SSH server (native PTY) and HTTP/WebSocket (xterm.js → SSH bridge) share the same Lobby and DB instances
- **Combat log reveal**: Logs shown incrementally via `PendingLogs` → `CombatLog` (one per UI tick)
- **Cooperative-leave shop**: `LeaveShop(fingerprint)` only flips the phase once every available player has signaled; the UI polls so non-leavers follow the transition
- **Achievement keying**: achievements / unlocks / titles / codex / daily records are keyed by **fingerprint**. Rankings and stats are keyed by **player name**

## Game Balance Constants

- 20 floors with mini-bosses at 4/9/14/19 and bosses at 5/10/15/20
- Monster scaling: 1.15× power per floor above minimum (regular monsters only; bosses use base stats)
- Solo mode halves enemy HP and DEF
- Cooperative bonus: +10% damage on the second-or-later attacker against the same target
- Inventory limit: 10 items per player; 3 skill uses per combat (mutated to 0 by the `no_skills` weekly mutation)
- Boss drops grant a random relic (10 types: 7 passive — HealOnKill / ATKBoost / DEFBoost / GoldBoost / PoisonImmunity / BurnResist / LifeSteal — + 3 active via `ActionUseRelic`, once per combat: SmokeBomb / Bloodstone / TimeGlass)
