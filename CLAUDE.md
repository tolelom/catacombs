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

Docker:
```bash
docker build -t catacombs .
docker-compose up                 # SSH on :2222, HTTP on :8080
```

## Architecture

**Package dependency flow:** `main` → `server`/`web`/`store` → `game` → `dungeon` → `entity` → `combat`

| Package | Responsibility |
|---------|---------------|
| `main.go` | Entry point: initializes BoltDB (`./data/catacombs.db`), starts SSH server (:2222) and HTTP server (:8080) |
| `game/` | Lobby (room management, player tracking, reconnection), GameSession (turn-based state), turn execution (5s action timeout), room events (combat/shop/treasure) |
| `ui/` | Bubble Tea state machine with 8 screen states (nickname → lobby → class select → game → shop → result → leaderboard → achievements). `model.go` is the central state machine (~19K lines) |
| `dungeon/` | BSP tree procedural generation (60x20 maps), ASCII rendering with floor themes, field-of-view |
| `entity/` | Player (4 classes: Warrior/Mage/Healer/Rogue), Monster (8 types + 4 bosses with floor scaling), Items/Relics |
| `combat/` | Damage calculation, monster AI targeting, cooperative damage bonus |
| `store/` | BoltDB persistence: profiles, rankings, achievements (10 unlockable) |
| `server/` | Wish SSH server with fingerprint-based auth |
| `web/` | HTTP + WebSocket bridge to SSH, embedded xterm.js frontend |

## Key Patterns

- **Concurrent session management**: Mutex-protected game state for multi-player synchronization (up to 4 players per room)
- **Turn-based action collection**: 5-second timeout window; players who don't submit default to "Wait"
- **SSH fingerprint reconnection**: Players reconnect to active sessions via `Lobby.activeSessions` fingerprint mapping
- **Dual access**: SSH server (native PTY) and HTTP/WebSocket (xterm.js) share the same Lobby and DB instances
- **Combat log reveal**: Logs shown incrementally via `PendingLogs` → `CombatLog` system

## Game Balance Constants

- 20 floors with bosses at 5, 10, 15, 20
- Monster scaling: 1.15x power per floor above minimum
- Solo mode halves enemy stats
- Cooperative bonus: +10% damage when 2+ players target same enemy
- Inventory limit: 10 items, 3 skill uses per combat
