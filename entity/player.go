package entity

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

type Player struct {
	Name        string
	Fingerprint string
	Class       Class
	HP, MaxHP   int
	ATK, DEF    int
	Gold        int
	Inventory   []Item
	Relics      []Relic
	Dead        bool
	Fled        bool
}

func NewPlayer(name string, class Class) *Player {
	stats := classBaseStats[class]
	return &Player{
		Name:  name,
		Class: class,
		HP:    stats.HP,
		MaxHP: stats.HP,
		ATK:   stats.ATK,
		DEF:   stats.DEF,
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
	return def
}
