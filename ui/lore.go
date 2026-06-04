package ui

// monsterLore returns flavor text for a monster name. Empty string means "no
// lore yet" and the codex falls back to the entry name only.
func monsterLore(name string) string {
	switch name {
	case "Slime":
		return "끈적이는 산 덩어리. 약하지만 무리짓는다."
	case "Skeleton":
		return "녹슨 검을 들고 다시 깨어난 자. 카타콤의 첫 경비병."
	case "Orc":
		return "광산 깊은 곳에서 자란 야만족. 한 방이 무겁다."
	case "Dark Knight":
		return "타락한 기사. 이름조차 잊혔다."
	case "Guardian":
		return "1대 보스. 지하 5층의 수호자, 늪의 독을 다룬다."
	case "Warden":
		return "2대 보스. 지하 10층의 간수. 화염 채찍을 휘두른다."
	case "Overlord":
		return "3대 보스. 빙하 협곡의 군주. 시간을 동결시킨다."
	case "Archlich":
		return "최종 보스. 카타콤의 진짜 주인. 죽음 자체."
	case "Mimic":
		return "보물 상자로 위장한 포식자. 욕심 많은 자만 잡힌다."
	}
	return ""
}

// itemLore returns flavor text for items / relics / set pieces.
func itemLore(name string) string {
	switch name {
	case "HP Potion":
		return "약초와 강물. 응급용."
	case "Vampiric Ring":
		return "한 모금의 피로 상처를 봉합한다."
	case "Power Amulet":
		return "기사단의 유품. 손목이 무거워질수록 강해진다."
	case "Iron Ward":
		return "방패 장인의 마지막 작품."
	case "Gold Charm":
		return "왜인지 금을 끌어모은다. 행운인지 저주인지."
	case "Antidote Charm":
		return "독사의 송곳니를 박아 만든 부적."
	case "Flame Guard":
		return "불 마법사가 자신의 망토를 짜 만든 부적."
	case "Life Siphon":
		return "고대 의식 도구. 적의 생명을 자기 것으로 바꾼다."
	}
	return ""
}

// eventLore returns flavor text for random events.
func eventLore(name string) string {
	switch name {
	case "altar":
		return "잊혀진 신의 제단. 무엇을 바칠 것인가."
	case "fountain":
		return "맑은 물이 흐른다. 마실 가치가 있을까."
	case "merchant":
		return "복도 한가운데 좌판을 깐 행상."
	case "trap_room":
		return "발 밑이 무너진다. 너무 늦었다."
	case "shrine":
		return "낡은 사당. 기도가 들릴지도."
	case "chest":
		return "잠긴 상자. 안에 뭐가 있을까."
	case "ghost":
		return "흐린 형체가 손짓한다. 도와줄까."
	case "mushroom":
		return "발광하는 버섯. 먹어볼까."
	case "gravekeeper":
		return "묘지기는 말이 없다. 무덤은 더 말이 없다."
	case "forge":
		return "망치 소리가 멎은 지 오래. 잔불만 남았다."
	case "toll_bridge":
		return "트롤의 다리. 요금은 협상 불가."
	case "feast":
		return "주인 없는 만찬. 초대받은 적 없는 손님."
	}
	return ""
}
