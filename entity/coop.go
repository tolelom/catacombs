package entity

type CoopAbility struct {
	Name string
	Desc string
}

var coopAbilities = map[Class]CoopAbility{
	ClassWarrior: {Name: "견고함", Desc: "1턴간 받는 피해 -75%."},
	ClassMage:    {Name: "결계", Desc: "1턴간 파티 받는 피해 -50%. 솔로: 자기에게만 적용."},
	ClassHealer:  {Name: "신성 가호", Desc: "죽은 아군 1명 부활(런 1회) 또는 파티 30% 회복(전투 1회)."},
	ClassRogue:   {Name: "정찰", Desc: "다음 방 종류 미리 공개. 층당 1회. 전투 외 사용."},
}

func GetCoopAbility(c Class) CoopAbility {
	return coopAbilities[c]
}
