package entity

import "fmt"

type Class int

const (
	ClassWarrior Class = iota
	ClassMage
	ClassHealer
	ClassRogue
)

func (c Class) String() string {
	return [...]string{"Warrior", "Mage", "Healer", "Rogue"}[c]
}

type classStats struct {
	HP, ATK, DEF int
}

var classBaseStats = map[Class]classStats{
	ClassWarrior: {120, 12, 8},
	ClassMage:    {70, 20, 3},
	ClassHealer:  {90, 8, 5},
	ClassRogue:   {85, 15, 4},
}

type StatusEffect int

const (
	StatusPoison StatusEffect = iota
	StatusBurn
	StatusFreeze
	StatusBleed
	StatusCurse
)

type ActiveEffect struct {
	Type     StatusEffect
	Duration int // remaining turns
	Value    int // damage per turn or effect strength
}

type Player struct {
	Name        string
	Fingerprint string
	Class       Class
	HP, MaxHP   int
	ATK, DEF    int
	Gold        int
	Inventory   []Item
	Relics      []Relic
	Effects     []ActiveEffect
	Dead        bool
	Fled        bool
	SkillUses   int // remaining skill uses this combat
	Skills      *PlayerSkills
}

func NewPlayer(name string, class Class) *Player {
	stats := classBaseStats[class]
	return &Player{
		Name:   name,
		Class:  class,
		HP:     stats.HP,
		MaxHP:  stats.HP,
		ATK:    stats.ATK,
		DEF:    stats.DEF,
		Skills: NewPlayerSkills(),
	}
}

func (p *Player) TakeDamage(dmg int) {
	p.HP -= dmg
	if p.HP <= 0 {
		p.HP = 0
		p.Dead = true
	}
}

func (p *Player) Heal(amount int) {
	for _, e := range p.Effects {
		if e.Type == StatusCurse {
			reduction := 100 - e.Value
			if reduction < 0 {
				reduction = 0
			}
			amount = amount * reduction / 100
			break
		}
	}
	p.HP += amount
	if p.HP > p.MaxHP {
		p.HP = p.MaxHP
	}
}

func (p *Player) IsDead() bool {
	return p.Dead
}

func (p *Player) IsOut() bool {
	return p.Dead || p.Fled
}

func (p *Player) Revive(hpPercent float64) {
	p.Dead = false
	p.HP = int(float64(p.MaxHP) * hpPercent)
	if p.HP < 1 {
		p.HP = 1
	}
}

func (p *Player) EffectiveATK() int {
	atk := p.ATK
	for _, item := range p.Inventory {
		if item.Type == ItemWeapon {
			atk += item.Bonus
		}
	}
	for _, r := range p.Relics {
		if r.Effect == RelicATKBoost {
			atk += r.Value
		}
	}
	atk += p.Skills.GetATKBonus(p.Class)
	atk += SetBonusATK(p.Inventory)
	return atk
}

func (p *Player) EffectiveDEF() int {
	def := p.DEF
	for _, item := range p.Inventory {
		if item.Type == ItemArmor {
			def += item.Bonus
		}
	}
	for _, r := range p.Relics {
		if r.Effect == RelicDEFBoost {
			def += r.Value
		}
	}
	def += p.Skills.GetDEFBonus(p.Class)
	def += SetBonusDEF(p.Inventory)
	return def
}

func (p *Player) AddEffect(e ActiveEffect) {
	// Check relic immunities
	for _, r := range p.Relics {
		if e.Type == StatusPoison && r.Effect == RelicPoisonImmunity {
			return // immune
		}
		if e.Type == StatusBurn && r.Effect == RelicBurnResist {
			e.Value = e.Value / 2 // halve burn damage
		}
	}
	// Don't stack same type, refresh duration
	for i, existing := range p.Effects {
		if existing.Type == e.Type {
			p.Effects[i] = e
			return
		}
	}
	p.Effects = append(p.Effects, e)
}

func (p *Player) HasEffect(t StatusEffect) bool {
	for _, e := range p.Effects {
		if e.Type == t {
			return true
		}
	}
	return false
}

func (p *Player) TickEffects() []string {
	var msgs []string
	remaining := p.Effects[:0] // reuse underlying array
	for i := 0; i < len(p.Effects); i++ {
		e := &p.Effects[i]
		switch e.Type {
		case StatusPoison:
			p.HP -= e.Value
			if p.HP <= 0 {
				p.HP = 1 // Poison can't kill, leaves at 1 HP
			}
			msgs = append(msgs, fmt.Sprintf("%s 독 피해 %d", p.Name, e.Value))
		case StatusBurn:
			p.HP -= e.Value
			if p.HP <= 0 {
				p.HP = 0
				p.Dead = true
			}
			msgs = append(msgs, fmt.Sprintf("%s 화상 피해 %d", p.Name, e.Value))
		case StatusFreeze:
			msgs = append(msgs, fmt.Sprintf("%s 동결됨!", p.Name))
		case StatusBleed:
			p.HP -= e.Value
			msgs = append(msgs, fmt.Sprintf("%s 출혈 피해 %d", p.Name, e.Value))
			// Bleed escalates per turn while a single application is active;
			// note that AddEffect replaces an existing Bleed rather than
			// stacking, so a fresh hit resets the intensification.
			e.Value++
		case StatusCurse:
			msgs = append(msgs, fmt.Sprintf("%s 저주 상태! 회복량 감소", p.Name))
		}
		if p.HP < 0 {
			p.HP = 0
		}
		e.Duration--
		if e.Duration > 0 {
			remaining = append(remaining, *e)
		}
	}
	p.Effects = remaining
	if p.HP <= 0 && !p.Dead {
		p.Dead = true
	}
	return msgs
}
