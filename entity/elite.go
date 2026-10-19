package entity

import (
	"math/rand"
)

type ElitePrefixType int

const (
	PrefixVenomous ElitePrefixType = iota
	PrefixBurning
	PrefixFreezing
	PrefixBleeding
	PrefixVampiric
)

type ElitePrefixDef struct {
	Name    string
	HPMult  float64
	ATKMult float64
	OnHit   StatusEffect
}

var ElitePrefixDefs = map[ElitePrefixType]ElitePrefixDef{
	PrefixVenomous: {"Venomous", 1.3, 1.0, StatusPoison},
	PrefixBurning:  {"Burning", 1.2, 1.1, StatusBurn},
	PrefixFreezing: {"Freezing", 1.2, 1.0, StatusFreeze},
	PrefixBleeding: {"Bleeding", 1.1, 1.2, StatusBleed},
	PrefixVampiric: {"Vampiric", 1.4, 1.1, StatusEffect(-1)},
}

func (p ElitePrefixType) String() string {
	return ElitePrefixDefs[p].Name
}

var defaultEliteRng = rand.New(rand.NewSource(1))

func RandomPrefix(rng *rand.Rand) ElitePrefixType {
	if rng == nil {
		rng = defaultEliteRng
	}
	return ElitePrefixType(rng.Intn(len(ElitePrefixDefs)))
}

func ApplyPrefix(m *Monster, prefix ElitePrefixType) {
	def := ElitePrefixDefs[prefix]
	m.IsElite = true
	m.ElitePrefix = prefix
	m.Name = def.Name + " " + m.Name
	m.HP = int(float64(m.HP) * def.HPMult)
	m.MaxHP = m.HP
	m.ATK = int(float64(m.ATK) * def.ATKMult)
}
