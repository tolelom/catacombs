package record

import (
	"log/slog"

	"github.com/tolelom/catacombs/entity"
	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/store"
)

type Recorder struct {
	db *store.DB
}

func New(db *store.DB) *Recorder {
	return &Recorder{db: db}
}

func (r *Recorder) RecordRun(result game.RunResult) {
	run := newRunFacts(result.State)
	for _, p := range result.State.Players {
		if p.Fingerprint == "" {
			continue
		}
		m := memberRecord{db: r.db, player: p, fp: p.Fingerprint, run: run}
		m.saveRun()
		m.awardSouls()
		m.countMastery()
		m.unlockMilestoneAchievements()
		m.unlockPersonalAchievements()
		m.countRunCounters()
		m.unlockCounterAchievements()
		m.unlockContent()
		m.earnTitles()
		if result.DailyMode {
			m.saveDaily(result.DailyDate)
		}
	}
}

type runFacts struct {
	state      game.GameState
	partyScore int
	members    []string
}

func newRunFacts(st game.GameState) runFacts {
	facts := runFacts{state: st}
	for _, p := range st.Players {
		facts.partyScore += p.Gold
		facts.members = append(facts.members, p.Name)
	}
	return facts
}

type memberRecord struct {
	db     *store.DB
	player *entity.Player
	fp     string
	run    runFacts
}

func (m memberRecord) saveRun() {
	st := m.run.state
	if err := m.db.SaveRun(m.player.Name, st.FloorNum, m.run.partyScore, m.player.Class.String(), m.run.members); err != nil {
		slog.Error("save run failed", "error", err, "player", m.player.Name, "floor", st.FloorNum)
	}
}

func (m memberRecord) awardSouls() {
	if soulGain := int64(m.player.Gold / 10); soulGain > 0 {
		if err := m.db.AddSouls(m.fp, soulGain); err != nil {
			slog.Error("soul award failed", "error", err, "fp", m.fp, "amount", soulGain)
		}
	}
}

func (m memberRecord) countMastery() {
	if m.run.state.FloorNum >= 5 {
		m.incCounter(store.MasteryCounterKey(m.player.Class.String()), 1)
	}
}

func (m memberRecord) unlockMilestoneAchievements() {
	st := m.run.state
	m.unlockIf(st.FloorNum >= 5, "first_clear")
	m.unlockIf(st.FloorNum >= 10, "floor10")
	m.unlockIf(st.Victory, "floor20")
	m.unlockIf(st.SoloMode && st.FloorNum >= 5, "solo_clear")
	m.unlockIf(st.BossKilled, "boss_slayer")
	m.unlockIf(st.FleeSucceeded, "flee_master")
	m.unlockIf(st.Victory && !st.AnyPlayerDied, "no_death")
	m.unlockIf(st.StartingPartySize >= 4, "full_party")
	m.unlockIf(len(st.CombosFiredThisRun) >= 1, "coop_first")
	m.unlockIf(countCoopCombos(st.CombosFiredThisRun) >= 6, "coop_master")
	m.unlockIf(st.MimicKilled, "mimic_bait")
	m.unlockIf(st.BossPhase2Reached, "boss_phase2")
}

func (m memberRecord) unlockPersonalAchievements() {
	m.unlockIf(m.player.Gold >= 200, "gold_hoarder")
	m.unlockIf(len(m.player.Relics) >= 3, "relic_collector")
	m.unlockIf(m.player.Skills != nil && m.player.Skills.Allocated >= 4, "tier4_unlocked")
}

func (m memberRecord) countRunCounters() {
	st := m.run.state
	for bossName := range st.BossesKilledThisRun {
		m.incCounter("boss_kill:"+bossName, 1)
	}
	if st.RevivesThisRun > 0 {
		m.incCounter("revives", int64(st.RevivesThisRun))
	}
	if st.AltarUsedThisRun > 0 {
		m.incCounter("altars", int64(st.AltarUsedThisRun))
	}
}

func (m memberRecord) unlockCounterAchievements() {
	if revives, err := m.db.GetCounter(m.fp, "revives"); err != nil {
		slog.Error("counter read failed", "error", err, "key", "revives")
	} else {
		m.unlockIf(revives >= 5, "revive_5")
	}
	if altars, err := m.db.GetCounter(m.fp, "altars"); err != nil {
		slog.Error("counter read failed", "error", err, "key", "altars")
	} else {
		m.unlockIf(altars >= 3, "cursed_lover")
	}
	if souls, err := m.db.GetSouls(m.fp); err != nil {
		slog.Error("souls read failed", "error", err)
	} else {
		m.unlockIf(souls >= 1000, "soul_collector")
	}
}

func (m memberRecord) unlockContent() {
	st := m.run.state
	m.unlockContentIf(st.FloorNum >= 10, "fifth_class")
	m.unlockContentIf(len(st.Players) >= 3 && st.FloorNum >= 5, "hard_mode")
	m.unlockContentIf(st.Victory, "mutations")
}

func (m memberRecord) earnTitles() {
	st := m.run.state
	m.earnTitleIf(true, "novice")
	m.earnTitleIf(st.FloorNum >= 5, "explorer")
	m.earnTitleIf(st.FloorNum >= 10, "veteran")
	m.earnTitleIf(st.Victory, "champion")
	m.earnTitleIf(st.Victory && !st.AnyPlayerDied, "survivor")
	m.earnTitleIf(m.player.Gold >= 500, "gold_king")
	if mp, err := m.db.CountMultiplayerRuns(m.player.Name); err != nil {
		slog.Error("multiplayer run count failed", "error", err, "player", m.player.Name)
	} else {
		m.earnTitleIf(mp >= 5, "team_player")
	}
}

func (m memberRecord) saveDaily(date string) {
	if err := m.db.SaveDaily(store.DailyRecord{
		Date:         date,
		Player:       m.fp,
		PlayerName:   m.player.Name,
		FloorReached: m.run.state.FloorNum,
		GoldEarned:   m.player.Gold,
	}); err != nil {
		slog.Error("save daily failed", "error", err, "date", date, "fp", m.fp)
	}
}

func (m memberRecord) unlockIf(cond bool, id string) {
	if !cond {
		return
	}
	if _, err := m.db.UnlockAchievement(m.fp, id); err != nil {
		slog.Error("achievement unlock failed", "error", err, "id", id)
	}
}

func (m memberRecord) unlockContentIf(cond bool, id string) {
	if !cond {
		return
	}
	if _, err := m.db.UnlockContent(m.fp, id); err != nil {
		slog.Error("content unlock failed", "error", err, "id", id)
	}
}

func (m memberRecord) earnTitleIf(cond bool, id string) {
	if !cond {
		return
	}
	if _, err := m.db.EarnTitle(m.fp, id); err != nil {
		slog.Error("title grant failed", "error", err, "id", id)
	}
}

func (m memberRecord) incCounter(key string, delta int64) {
	if err := m.db.IncCounter(m.fp, key, delta); err != nil {
		slog.Error("counter increment failed", "error", err, "key", key)
	}
}

func countCoopCombos(fired map[string]bool) int {
	coop := []string{"Iron Aegis", "Crusader", "Expose Weakness", "Sanctuary", "Mirage", "Venomous Edge"}
	n := 0
	for _, name := range coop {
		if fired[name] {
			n++
		}
	}
	return n
}
