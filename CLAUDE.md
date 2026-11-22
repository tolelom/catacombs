# CLAUDE.md

Guidance for Claude Code in this repository. Package map and the rules every change must follow are in `ARCHITECTURE.md`. Build, test and deploy are in `CONTRIBUTING.md`.

## Verify

Run the narrowest test first, then the full suite and `go vet ./...` before finishing.

```bash
go test ./combat/                 # one package
go test ./entity/ -run TestName   # one test
```

## Code rules

- Every RNG-consuming entry point in `combat/` takes a leading `*rand.Rand`. Nil falls back to a process-global for tests; game code passes the session RNG `s.rng`.
- The weekly `Mutation` is applied at read time through `effectiveSkillUses` and `effectiveTurnTimeout`. Do not write a mutation into the shared config.
- Balance values (turn timeout, inventory limit, skill uses, monster scaling, solo and hard-mode multipliers) come from `config/config.go` `defaults()` and `config.yaml`. Read them from `cfg` instead of adding literals.
- Connected players are never removed for idling. Only players disconnected longer than `disconnectGrace` are dropped; `GameSession.clock` is injectable to test that path.
- `RunTurn` resets the action map and drains pending player-left signals before the collection window. Players who submit nothing get `ActionWait`; dead or fled players are skipped.
- `Lobby.LeaveSession(code, fingerprint)` removes one player. The room and combat loop tear down only when the last player leaves.
- `LeaveShop(fingerprint)` flips the phase only after every available player has left. The UI polls so the others follow.
- Combat logs move from `PendingLogs` to `CombatLog` one entry per UI tick.
