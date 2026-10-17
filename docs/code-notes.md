# Code notes

Explanations that used to live in production-code comments, grouped by file and declaration. The code no longer carries comments (project convention); keep design notes here instead. Moved verbatim on 2026-10-16, so a note may describe the code as it was at that point.

## game/actions.go

- `GameSession.SubmitAction`: Prevent duplicate submissions in the same turn
- `GameSession.RevealNextLog`: RevealNextLog moves one log from PendingLogs to CombatLog. Returns true if there was one to reveal.
- `GameSession.AllocateSkillPoint`: AllocateSkillPoint spends one skill point into the given branch for the player.
- `GameSession.AllocateSkillPoint`: MaxHPBoost nodes raise both MaxHP and current HP by the delta.
- `GameSession.SendChat`: SendChat appends a chat message to the combat log

## game/event.go

- `GameSession.enterRoomLocked`: enterRoomLocked performs room entry logic. Caller must hold s.mu.
- `GameSession.enterRoomLocked`: room.Cleared flipped after all picks resolve in resolveEventVote
- `GameSession.enterRoomLocked`: room.Cleared flipped after vote quorum in resolveEventVote
- `GameSession.handleMimic`: handleMimic: 50% it's actually a treasure, 50% an ambush by a tough monster (1.5x stat scaling). The room is marked Cleared on the treasure path; combat path defers to combat clear.
- `GameSession.handleGamble`: handleGamble: each player can bet 50 gold. Outcome: - 50%: lose the 50 gold - 30%: win 100 gold (+50 net) - 20%: win a random item
- `GameSession.handleCursedAltar`: handleCursedAltar: each player loses 30% MaxHP, gains random permanent stat boost (ATK+5 / DEF+3 / Gold+100).
- `GameSession.handleWanderingMerchant`: handleWanderingMerchant: rare items at 2x normal price (one-time stock).
- `GameSession.applySoloScaling`: applySoloScaling weakens a monster for single-player runs: HP and DEF are reduced by SoloHPReduction (squishier) and ATK by SoloATKReduction (hits softer). The ATK cut compensates for a solo party having no healer sustain or coop combos — without it, fragile classes cannot survive late bosses long enough to kill them. No-op outside solo mode.
- `GameSession.earlyThreatMult`: earlyThreatMult returns a HP/ATK multiplier that compresses the trivial early-game "warmup". It is a smooth bump: 1.0 on the onboarding floors (1-2), rising to a mid-early peak (~floor 6), then tapering back to 1.0 by floor 10 where the exponential min-floor scaling takes over. This brings tension forward to ~floor 4-5 without creating a difficulty discontinuity at floor 10. EarlyThreatRamp == 0 disables it entirely.
- `GameSession.earlyThreatMult`: ramp returns to 1.0 here, merging into base scaling
- `GameSession.applyEarlyThreat`: applyEarlyThreat scales a regular monster's HP and ATK (not DEF — the subtractive damage formula makes DEF lifts disproportionately punishing) by earlyThreatMult. No-op once the multiplier is 1.0.
- `GameSession.resetPerCombatStateLocked`: resetPerCombatStateLocked refreshes every map and flag scoped to a single combat encounter: skill uses, coop-used, active-relic-used, Time Glass extra actions, the solo chain sequence, plus the pending Venomous Edge poison application. Without the extraActionNext reset, Time Glass would carry a free attack from the prior combat into this one. Every combat-entry site (spawnMonsters, spawnBoss, spawnMiniBoss, handleMimic) calls this so adding a new per-combat flag means editing one place, not four. Caller holds s.mu.
- `GameSession.spawnBoss`: Swamp theme
- `GameSession.spawnBoss`: Volcano theme
- `GameSession.spawnBoss`: Glacier theme
- `GameSession.spawnBoss`: Inferno theme (+ natural AoE every 3 turns)
- `GameSession.applyPoisonNextCombatIfPending`: applyPoisonNextCombatIfPending applies the Venomous Edge combo's effect at combat start: every spawned monster gets a 3-turn poison stack. Caller holds s.mu.
- `GameSession.grantTreasure`: 10% chance: drop a set item instead of regular gear.
- `GameSession.generateShopItems`: Weapon bonus scales: base 3-8 + floor/3
- `GameSession.generateShopItems`: Armor bonus scales: base 2-5 + floor/4
- `GameSession.generateShopItems`: Prices scale with power
- `GameSession.generateShopItems`: Potion heals more on higher floors
- `GameSession.applyEventOutcome`: applyEventOutcome applies an event outcome to the party per the rules in the design spec: HPChange to each alive player, GoldChange split evenly (remainder to the first alive player), ItemDrop one rolled item per alive player (skip if inventory full). Caller holds s.mu.
- `GameSession.applyEventOutcome`: Lucky charm: one charm voids ALL of a player's losses from a single gambled outcome (one result = one insurance claim — same invariant as applyOutcomeToPlayer). Consumed once here, then both the HP and gold loss branches honor the protection.
- `GameSession.applyEventOutcome`: HP change: each alive player heals/takes damage of the full amount.
- `GameSession.applyEventOutcome`: Uniform with combat: try solo-healer auto-revive, otherwise log the rip and flip AnyPlayerDied.
- `GameSession.applyEventOutcome`: Gold change: split evenly across alive; remainder to alive[0].
- `GameSession.applyEventOutcome`: Charm holders skip their share of a gambled gold loss (their remainder portion is voided, not redistributed).
- `GameSession.applyEventOutcome`: Concrete item grant: each alive player gets their own copy. Full inventories convert to bonus*5 gold — same invariant as applyOutcomeToPlayer. Mutually exclusive with ItemDrop so an outcome accidentally setting both can't double-grant.
- `GameSession.resolveEventVoteLocked`: resolveEventVoteLocked tallies eventVotes, picks the winning choice (lowest index breaks ties), runs Resolve, applies the outcome party-wide, and returns the session to PhaseExploring with the room cleared. Per-player pending events (loot picks) skip the tally and resolve each player's own choice individually. Caller holds s.mu and has verified quorum.
- `GameSession.resolveEventVoteLocked`: Tally
- `GameSession.triggerEvent`: triggerEvent picks a random event and parks the session in PhaseEvent awaiting player votes. The actual outcome runs later via resolveEventVote. Caller holds s.mu.
- `GameSession.triggerEvent`: Edge case: nobody alive to vote (post-wipe race). Skip + clear.
- `GameSession.triggerLoot`: triggerLoot presents the treasure-room loot table and parks the session in PhaseEvent with per-player resolution: each player's own pick applies to them only. Mirrors triggerEvent's zero-alive guard. Caller holds s.mu.
- `GameSession.startEventPhaseLocked`: startEventPhaseLocked stashes the typed event, builds the snapshot-safe view, and parks the session in PhaseEvent awaiting player input. perPlayer selects individual resolution (loot picks) over party majority (random events). Caller holds s.mu and has verified at least one alive player.
- `GameSession.resolveLootPicksLocked`: resolveLootPicksLocked applies each alive player's own pick. Iterates s.state.Players (not the vote map) so RNG consumption order is deterministic for daily runs. Voteless alive players (disconnect-sweep edge) are skipped. Caller holds s.mu.
- `GameSession.applyOutcomeToPlayer`: applyOutcomeToPlayer applies a loot outcome to a single player. Unlike applyEventOutcome (party-wide), gold goes entirely to the picker, and a concrete Item grant converts to bonus*5 gold when the inventory is full (mirrors handleGamble's jackpot fallback; consistent with the shop price formula 40+bonus*5). Loot outcomes always carry concrete items (o.Item), never deferred ItemDrop rolls — see buildRotatingLootChoice. A player killed by the HP cost still receives the item/gold: pick and price are one transaction, and a revive keeps the loot. Caller holds s.mu.
- `GameSession.applyOutcomeToPlayer`: Lucky charm: one charm voids ALL losses of a single gambled outcome (one result = one insurance claim). Fixed costs pass through.
- `GameSession.grantSecretTreasure`: Double treasure: grant two items per player
- `GameSession.spawnMiniBoss`: Use same pattern as the subsequent boss
- `GameSession.applyDifficultyMods`: applyDifficultyMods scales a monster's HP/ATK by HardMode, NG+ level, and the active daily challenge modifier. SoloMode is intentionally NOT applied here — solo balance is handled per-spawn-context (regular spawns and bosses reduce HP for solo, but B10 phase-2 summons stay tough by design). Caller holds s.mu.

## game/ngplus.go

- `Curse`: Curse is an end-game handicap with an upside, applied at NG+ start.
- `Curse`: Multipliers (1.0 = no change). Applied at AddPlayer / spawn.
- `Curse`: added to per-combat skill uses (negative reduces)
- `Curse`: applied to gold rewards
- `AvailableCurses`: AvailableCurses lists the 5 NG+ curses; one is rolled at NG+ start.
- `PickCurse`: PickCurse returns one curse using the supplied RNG.

## game/session.go

- `GameSession`: playerName -> action
- `GameSession`: wakes RunTurn's collect loop on mid-turn quit
- `GameSession`: fingerprint -> voted room index
- `GameSession`: fingerprint -> wants to leave the shop
- `GameSession`: fingerprint → choice index
- `GameSession`: typed event with Resolve funcs; not snapshotted
- `GameSession`: pendingEvent resolves per player (loot) vs party majority
- `GameSession`: session-scoped RNG; deterministic for daily runs
- `GameSession`: Coop ability state — all maps keyed by player fingerprint.
- `GameSession`: reset on combat start
- `GameSession`: run-scoped (revive arm of Divine Grace)
- `GameSession`: reset on floor advance
- `GameSession`: run-scoped solo auto-revive
- `GameSession`: applies for current turn only
- `GameSession`: applies for next turn (Sanctuary combo)
- `GameSession`: Smoke Bomb: extra dodge%, fingerprint→bonus (0..1)
- `GameSession`: Time Glass: bearer gets a second action next turn
- `GameSession`: fingerprint → already used active relic this combat
- `GameSession`: populated by Foresight, consumed at next room entry
- `GameSession`: next turn party crit (Expose Weakness combo)
- `GameSession`: next turn 50% enemy attacks miss (Mirage)
- `GameSession`: next combat starts enemies poisoned (Venomous Edge)
- `GameSession`: Solo chain state — per-combat action sequence for SoloMode chains (the solo replacement for cross-class party combos).
- `GameSession`: previous turn's successful action ("" breaks chains)
- `GameSession`: consecutive attacks so far this combat
- `GameSession`: The fields below are set during session construction (inside an EnsureSession init callback under Lobby.mu) and are not mutated thereafter. Reading them without s.mu is safe because the lobby-lock release establishes happens-before for every subsequent reader.
- `GameSession`: New Game+ state — same publication invariant as above.
- `GameSession`: Daily challenge modifier (set by lobby when DailyMode = true) — same publication invariant as above.
- `GameSession.hasMutation`: hasMutation returns true if the session has the given mutation active.
- `GameSession.reseedForFloor`: reseedForFloor refreshes the session RNG when entering a new floor. In DailyMode the seed is derived from (date + floor) so two players running the same daily on the same date face identical monster spawns, item rolls, and event outcomes. Outside DailyMode we fold in the floor number so each floor still gets fresh randomness rather than reusing the prior floor's stream. Callers must hold s.mu.
- `GameSession.ApplyWeeklyMutation`: ApplyWeeklyMutation sets the current week's mutation on this session.
- `GameSession.effectiveSkillUses`: effectiveSkillUses returns the per-combat skill use budget, accounting for the active mutation and any NG+ curse. Reading via this helper avoids mutating shared config.
- `GameSession.effectiveTurnTimeout`: effectiveTurnTimeout returns the per-turn timeout in seconds, accounting for the active mutation.
- `GameSession.StartGame`: StartGame determines solo mode from actual player count at game start
- `GameSession.combatLoop`: combatLoop continuously runs turns while in combat phase
- `GameSession.combatLoop`: Reading state without s.mu here is safe: FloorNum/Victory are only written inside RunTurn (advanceFloor / turn.go), all on this same combatLoop goroutine.
- `GameSession.combatLoop`: Wake periodically so the inactivity sweep above runs even during PhaseShop / PhaseExploring. Otherwise a disconnected player can deadlock the shop-leave consensus or move-vote (both require every alive fingerprint to participate) until the next combat — which never arrives without the move.
- `GameSession.RemovePlayer`: RemovePlayer removes a player from the session by fingerprint and returns the number of remaining players. Returns -1 if the fingerprint was not found (so callers can distinguish a no-op from a successful drop to zero).
- `GameSession.RemovePlayer`: The leaver might have been the last voter the party was waiting on — re-evaluate the current phase's quorum so we don't deadlock.
- `GameSession.RemovePlayer`: Wake RunTurn's collect loop in case it's blocked waiting for this player's never-arriving action.
- `GameSession.AddPlayer`: Dedup by fingerprint — a player who left the waiting room and rejoined must not be added twice to the session's player list.
- `GameSession.StartFloor`: Revive dead players at 30% HP

## game/shop.go

- `BuyResult`: BuyResult describes the outcome of a shop purchase attempt.
- `GameSession.BuyItem`: BuyItem handles shop purchases
- `GameSession.BuyItem`: Only gear sells out — potions and charms restock freely.
- `GameSession.tallyShopLeaversLocked`: tallyShopLeaversLocked returns true when every available (non-out) player has signaled they are ready to leave the shop. Treats a zero- available party as ready so a wipe in the shop doesn't deadlock. Empty-fingerprint players are counted through the "" key rather than skipped, so their quorum slot stays fillable. Caller holds s.mu.
- `GameSession.LeaveShop`: LeaveShop marks the calling player as ready to leave the shop. The phase only flips back to PhaseExploring once every available (non-out) player has pressed leave, so one impatient player can't yank others out mid-browse. Returns true if the call triggered the actual transition.
- `GameSession.LeaveShop`: Empty-fingerprint players (legacy/test path) signal under the "" key, mirroring move votes — excluding them entirely would make their quorum slot unfillable and deadlock the shop for the whole party.

## game/state.go

- `EventChoiceView`: EventChoiceView is a snapshot-safe view of one event choice for the UI. The Resolve function is intentionally omitted — it is non-copyable behavior state and lives only on the typed *RandomEvent in the session.
- `SoloChainView`: SoloChainView surfaces the solo-chain sequence state for the HUD. Populated only during solo-mode combat; nil otherwise.
- `PendingEventView`: PendingEventView is a snapshot-safe view of the active event, populated while Phase == PhaseEvent and nil otherwise.
- `PendingEventView`: true for loot picks: each player's own choice applies to them
- `RoomPreview`: RoomPreview is what Foresight reveals to the party about the next room before they vote to enter it. Boss rooms hide the monster names but expose the room type.
- `GameState`: reset per combat encounter
- `GameState`: recent combat messages
- `GameState`: fingerprint -> action description
- `GameState`: logs waiting to be revealed one by one
- `GameState`: true while logs are being replayed
- `GameState`: true once any player has died this run; powers the no_death achievement
- `GameState`: number of players at StartGame; powers the full_party achievement
- `GameState`: name of the most recent random event (for codex)
- `GameState`: fingerprint -> voted room index (exploration)
- `GameState`: active event during PhaseEvent
- `GameState`: fingerprint → choice index
- `GameState`: parallel to ShopItems; true once a weapon/armor slot is sold (consumables and charms restock freely)
- `GameState`: fingerprint -> wants to leave the shop (UI display only)
- `GameState`: populated when Foresight has been used
- `GameState`: fingerprint → can use coop now (HUD display)
- `GameState`: solo-mode combat chain state (HUD display)
- `GameState`: Run-scoped event flags (drives polish achievements at GameOver)
- `GameState`: combo name → fired at least once this run
- `GameState`: any boss reached Phase 2
- `GameState`: any mimic monster defeated
- `GameState`: cursed altar uses this run
- `GameState`: Divine Grace revives this run
- `GameState`: boss name → killed this run
- `GameSession.GetState`: Deep copy players. Skip allocating zero-length slices (most players have no Effects between combats, fresh players have no Inventory or Relics). At ~5 Hz polling per client these allocations dominate idle CPU otherwise.
- `GameSession.GetState`: Deep copy monsters
- `GameSession.GetState`: Deep copy floor
- `GameSession.GetState`: Copy combat log only when non-empty.
- `GameSession.GetState`: Copy submitted actions only when non-empty.
- `GameSession.GetState`: Copy move votes
- `GameSession.GetState`: Copy event votes
- `GameSession.GetState`: Copy pending event view (deep — Choices slice + per-Choice Tags slice)
- `GameSession.GetState`: Mirror shopLeavers for the UI's "N/M ready to leave" display.
- `GameSession.GetState`: Copy pending logs
- `GameSession.GetState`: Copy shop wares
- `GameSession.soloChainView`: soloChainView builds the HUD snapshot of the solo-chain state, or nil when it doesn't apply (party mode or out of combat). Caller holds s.mu.
- `GameSession.computeCoopAvailable`: computeCoopAvailable returns a map of fingerprint→true when that player can fire their coop ability right now. Caller must hold s.mu.

## game/turn.go

- `GameSession.RunTurn`: Clear single-turn coop modifiers from the previous turn before this turn resolves.
- `GameSession.RunTurn`: Collect actions with timeout
- `GameSession.RunTurn`: Recompute the alive set every iteration so a mid-turn quit (RemovePlayer drops the leaver from s.actions and pokes playerLeftCh) shrinks the expected count instead of stalling the party until the timeout fires.
- `GameSession.RunTurn`: loop re-evaluates aliveCount on next iteration
- `GameSession.RunTurn`: Default action for players who didn't submit: Wait
- `GameSession.RunTurn`: Time Glass: players flagged on the *previous* turn get one extra attack before this turn's normal player phase. Flag is cleared so a single use only buys one bonus turn.
- `GameSession.RunTurn`: One-turn flags consumed during this turn — clear so they don't carry over.
- `GameSession.RunTurn`: B15 phase 2 vulnerable window — flips every turn so the 1.5x damage buff alternates (per the design comment in checkBossPhaseTransitions); without this, Vulnerable would stay true permanently after entry.
- `GameSession.RunTurn`: PendingLogs now contains all turn results — UI will reveal them one by one via RevealNextLog
- `GameSession.advanceFloor`: Grant 1 skill point per floor clear
- `GameSession.grantBossRelic`: Bosses also drop one set item per surviving player.
- `randomSetItem`: randomSetItem rolls a random set item appropriate for the current floor.
- `GameSession.resolveMonsterActions`: Boss/mini-boss special pattern
- `GameSession.resolveMonsterActions`: Elite on-hit effects only apply when the strike actually connected — dodge/block/Sanctuary that reduced dmg to 0 shouldn't still inflict status (or heal vampiric for 0).
- `GameSession.pickHealTarget`: pickHealTarget returns the player a heal should land on: the explicitly chosen ally if it is alive and actually wounded, otherwise the most-wounded alive ally. Returns nil when no alive player needs healing — the caller then converts the heal into an offensive smite.
- `GameSession.firstAliveMonsterIdx`: firstAliveMonsterIdx returns the index of the first living monster, or -1 if none are alive.
- `GameSession.applyDamageMods`: applyDamageMods reduces incoming damage by coop reductions, returns 0 if invincible. Also applies class-based dodge/block rolls (Rogue +10% dodge, Warrior +10% block) and Smoke Bomb relic dodge bonus. Returns the final damage value (0 if dodged). Caller holds s.mu.
- `GameSession.runExtraActions`: runExtraActions performs one extra auto-attack per player who used Time Glass last turn. Skips if no monsters alive. Caller holds s.mu.
- `GameSession.runExtraActions`: Find first alive monster
- `GameSession.applyActiveRelic`: applyActiveRelic resolves an ActionRelic submission. choice is the UI's armed-relic cursor and selects which active relic in the player's inventory fires when more than one is owned. Once per combat per player. Caller holds s.mu.
- `GameSession.applyCoopAbility`: applyCoopAbility resolves one player's coop ability use. Caller holds s.mu.
- `GameSession.recordSoloAction`: recordSoloAction updates the solo-chain sequence state after a player's combat action resolves. actionType must reflect what actually happened — failed skills/coops, items-with-empty-bag, waits, freezes, and relic uses record "" and break every chain. No-op outside solo mode (party combos own that space). Caller holds s.mu.
- `GameSession.mergeCoopReduction`: mergeCoopReduction merges incoming damage-reduction multipliers into the session map, taking the maximum per fingerprint.
- `GameSession.checkBossPhaseTransitions`: checkBossPhaseTransitions runs after damage to detect bosses entering Phase 2 and applies their unique enrage effects. Threshold defaults to 50% HP, raised to 70% by the Cruel curse. Caller holds s.mu.
- `GameSession.checkBossPhaseTransitions`: B5 Guardian: berserk — ATK +50%, no more healing pattern
- `GameSession.checkBossPhaseTransitions`: B10 Warden: split — summon 2 slimes scaled to floor. applyDifficultyMods covers HardMode/NG+/DailyMod so summons stay scary in late-game runs (the HP*2 boost happens BEFORE scaling so it survives multiplication).
- `GameSession.checkBossPhaseTransitions`: tougher summons
- `GameSession.checkBossPhaseTransitions`: B15 Overlord: vulnerable window every other turn (toggled in MonsterAI)
- `GameSession.checkBossPhaseTransitions`: B20 Archlich: superchaos — all status patterns + ATK +30%, DEF +20%
- `GameSession.applyCoopComboEffect`: applyCoopComboEffect propagates extended coop combo effects to session state. Caller holds s.mu.
- `GameSession.handlePlayerDeathLocked`: handlePlayerDeathLocked handles a player's death uniformly: if the once-per-run solo-healer auto-revive applies, restores the player to 30% HP and logs the revive; otherwise logs the rip line and flips AnyPlayerDied for the no_death achievement. Burning the revive on event damage is intentional — any death counts per the Divine Grace design. Caller holds s.mu. Only call when p.IsDead() == true.

## game/votes.go

- `GameSession.recheckPhaseQuorumLocked`: recheckPhaseQuorumLocked runs the per-phase quorum tally and resolves the phase if quorum is met. Used by RemovePlayer and the combatLoop inactivity sweep so a player leaving (explicit Q or 60s timeout) cannot strand the party waiting for a vote they will never cast. Caller holds s.mu.
- `GameSession.recheckPhaseQuorumLocked`: Mirror the sweep behavior: prune any stale votes for fingerprints no longer in the alive set (RemovePlayer already deletes from the maps but defense-in-depth keeps the tally correct if a future caller forgets).
- `GameSession.SubmitForesight`: SubmitForesight is the Rogue's coop ability. Available once per floor while exploring. Reveals the next room's type and exposes monster room context for informed party voting. Boss rooms hide monster names but show "보스" type.
- `GameSession.tallyMoveQuorumLocked`: tallyMoveQuorumLocked returns the winning room index and true if every alive player has cast a move vote. Returns (-1, false) when quorum is not met or no alive players remain. Tie-break: lowest room index. Caller holds s.mu.
- `GameSession.SubmitMoveVote`: SubmitMoveVote records a player's room choice during exploration. When all alive players have voted, the majority choice wins and the party moves. Returns true if the vote triggered a move (all votes collected).
- `GameSession.SubmitMoveVote`: Mirror to snapshot map for UI display.
- `GameSession.tallyEventQuorumLocked`: tallyEventQuorumLocked returns true if every alive player has cast an event vote. Does NOT compute a winner — winner selection lives in resolveEventVoteLocked. Returns false when no alive players remain. The anonymous-solo special case stays in SubmitEventVote because it depends on the caller's fingerprint context. Caller holds s.mu.
- `GameSession.SubmitEventVote`: SubmitEventVote records a player's event choice during PhaseEvent. When all alive players have voted, the majority choice (ties broken by lowest index) is resolved and applied to the party. Returns true if this call triggered resolution.
- `GameSession.SubmitEventVote`: Mirror to snapshot map
- `GameSession.SubmitEventVote`: Defensive: nobody alive to receive the outcome. Clear and bail.
- `GameSession.SubmitEventVote`: Anonymous single-player test path: the caller passed no fingerprint so the vote did not land in s.eventVotes — treat this call as the sole alive vote.
- `GameSession.clearPendingEventLocked`: clearPendingEventLocked clears all pending-event state and optionally marks the current event room as cleared. Caller holds s.mu.

## ui/class_view.go

- `ClassSelectScreen`: ClassSelectScreen lets the player choose a class before entering the game.

## ui/context.go

- `Context`: Context holds shared state accessible to all screens.

## ui/event_view.go

- `tagIcon`: tagIcon maps a ChoiceTag to a short display token used in event hints.
- `EventScreen`: EventScreen handles the random-event vote phase. Mirrors ShopScreen's poll-based update pattern so other players' votes show up in real time.
- `EventScreen.submitVote`: Vote triggered resolution; the next tick will follow phase out.

## ui/game_view.go

- `GameScreen`: GameScreen handles the main gameplay: exploration, combat, and chat.
- `GameScreen`: for Healer skill targeting allies
- `GameScreen`: index into local player's active relics; cycled with [r]
- `file`: countCoopCombos returns how many of the 6 coop pair combos appear in the run-scoped combo set.
- `playerActiveRelics`: playerActiveRelics returns the local player's active (triggerable) relics in inventory order. Used by the [r] cycle key and the HUD armed-relic readout.
- `renderMap`: Count explored rooms
- `soloChainHint`: soloChainHint renders the one-line solo-chain status for the combat HUD, or "" when nothing is armed. Nil chain (party mode / out of combat) shows nothing.
- `renderEnemyPanel`: ASCII art
- `renderEnemyPanel`: Name + HP
- `file`: --- best-effort persistence helpers --------------------------------------- These store writes fire from the UI tick and have no user-facing error channel; silently dropping them meant lost codex entries, souls, and achievements with zero trace. Logging keeps failures visible to the operator without interrupting play.

## ui/leaderboard_view.go

- `LeaderboardScreen`: LeaderboardScreen shows the top runs.
- `markIfMine`: markIfMine wraps a leaderboard line with a highlight when the row belongs to the calling user, using the provided id (player name for ranking records, fingerprint for daily records) to match.
- `renderLeaderboard`: Tab header
- `renderLeaderboard`: By Floor
- `renderLeaderboard`: By Gold
- `renderLeaderboard`: Daily
- `renderLeaderboard`: Daily records are keyed by fingerprint, not name.

## ui/lobby_view.go

- `LobbyScreen`: LobbyScreen shows available rooms and lets players create/join.
- `LobbyScreen`: dailyDate is captured on lobby refresh so the header banner and the daily-challenge session created via [D] use the same date — avoids a midnight crossover where header shows yesterday's modifier but the run is created with today's.
- `LobbyScreen.Update`: Join-by-code input mode
- `LobbyScreen.Update`: Normal lobby key handling
- `LobbyScreen.Update`: Joining a room we just created shouldn't fail, but if it does the player must see why instead of a ghost lobby.
- `LobbyScreen.Update`: Daily Challenge: create a private solo daily session
- `renderLobby`: Show players in selected room

## ui/nickname_view.go

- `nicknamePhase`: nicknamePhase tracks the current step of the nickname/login screen.
- `phaseNickname`: entering nickname
- `phaseNickname`: existing account — enter password
- `phaseNickname`: new account — enter password
- `phaseNickname`: new account — confirm password
- `NicknameScreen`: NicknameScreen handles player name input and optional web login.
- `isWebUser`: isWebUser returns true when the player connected via the web bridge (no real SSH fingerprint). The bridge mints an "anon:<rand>" token in server/ssh.go; the title screen uses the same shape as a defensive fallback. Anything that isn't a real SHA256: key needs the password flow so progress can be saved.
- `NicknameScreen.Update`: Esc always goes back one step or cancels.
- `NicknameScreen.updateNickname`: SSH users with a real fingerprint skip password entirely.
- `NicknameScreen.updateNickname`: Web user — need password flow.
- `NicknameScreen.updatePasswordLogin`: Set deterministic fingerprint for web user.
- `minPasswordLen`: minPasswordLen is the floor for new web account passwords. Web users do not have an SSH key tying them to a fingerprint, so the password is the only barrier to nickname squatting once an account exists.
- `NicknameScreen.finishLogin`: finishLogin sets the player name, saves the profile, and transitions to lobby.
- `NicknameScreen.finishLogin`: Check for active session to reconnect.
- `webFingerprint`: webFingerprint produces a deterministic fingerprint for a web user.

## ui/result_view.go

- `ResultScreen`: ResultScreen shows the end-of-run summary and rankings.
- `ResultScreen.Update`: NG+ branch: leave current session, create new one with NGPlusLevel++ and a random curse
- `findPlayerRank`: findPlayerRank returns the 1-indexed position of the player's best run in the supplied ranking slice, or 0 if they don't appear in the top N.
- `renderResult`: Title
- `renderResult`: Player summary
- `renderResult`: Rankings
- `renderResult`: Highlight the current player's row so they can find themselves.

## ui/shop_view.go

- `ShopScreen`: ShopScreen handles the shop between floors.
- `ShopScreen.Update`: Another player may have just left the shop and triggered the transition; pick that up so we follow them out.
- `renderShop`: Show current player's gold
- `renderShop`: One-line explainer when a charm is on the shelf — the label alone doesn't tell players what counts as a "gamble".
- `renderShop`: Two lines: the single-line version overflows 80-col terminals.

## ui/souls_view.go

- `SoulsScreen`: SoulsScreen lets the player spend accumulated souls on permanent upgrades.
- `NewSoulsScreen`: NewSoulsScreen builds a fresh souls/upgrades menu.
- `SoulsScreen.Update`: Update handles input on the souls screen.
- `SoulsScreen.View`: View renders the souls screen.

## ui/title.go

- `TitleScreen`: TitleScreen is the main menu screen.
- `TitleScreen.Update`: Defensive only — server/ssh.go always supplies a real SHA256:... or anon:<rand> fingerprint in production. Match the production "anon:" prefix so downstream isWebUser() and lobby bookkeeping see one consistent shape.
- `TitleScreen.Update`: First time player — show nickname input
- `TitleScreen.Update`: Check for active session to reconnect

## ui/waiting_view.go

- `WaitingScreen`: WaitingScreen shows room members and lets players ready up before starting.
- `WaitingScreen.Update`: Check if all players are ready → start game
- `WaitingScreen.Update`: Solo: if only 1 player in room, start immediately
- `WaitingScreen.Update`: Leave room — unready, drop from lobby roster, and pull our fingerprint out of the session so an in-flight StartGame doesn't carry a ghost player into the run.
