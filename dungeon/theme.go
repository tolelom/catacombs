package dungeon

import "github.com/tolelom/catacombs/entity"

type ThemeModifier struct {
	Name        string
	StatusBoost entity.StatusEffect
	DamageMult  float64
	Description string
}

var themeModifiers = []ThemeModifier{
	{"Swamp", entity.StatusPoison, 1.5, "Toxic marshes amplify poison"},
	{"Volcano", entity.StatusBurn, 1.5, "Volcanic heat intensifies burns"},
	{"Glacier", entity.StatusFreeze, 1.5, "Glacial cold strengthens frost"},
	{"Inferno", entity.StatusEffect(-1), 1.3, "Hellfire empowers all afflictions"},
}

func GetTheme(floor int) ThemeModifier {
	switch {
	case floor <= 5:
		return themeModifiers[0]
	case floor <= 10:
		return themeModifiers[1]
	case floor <= 15:
		return themeModifiers[2]
	default:
		return themeModifiers[3]
	}
}
