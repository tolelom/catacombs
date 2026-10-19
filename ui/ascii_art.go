package ui

import "github.com/tolelom/catacombs/entity"

func MonsterArt(mt entity.MonsterType) []string {
	switch mt {
	case entity.MonsterSlime:
		return []string{
			` /\OO/\ `,
			` \    / `,
			`  |__| `,
		}
	case entity.MonsterSkeleton:
		return []string{
			` ,--. `,
			` |oo| `,
			` /||\ `,
		}
	case entity.MonsterOrc:
		return []string{
			` .---. `,
			`/o   o\`,
			`| --- |`,
		}
	case entity.MonsterDarkKnight:
		return []string{
			` /|||\ `,
			` |===| `,
			` |   | `,
		}
	case entity.MonsterBoss5:
		return []string{
			` /\  /\ `,
			`| @  @ |`,
			`|  ||  |`,
			`| \__/ |`,
			` \    / `,
		}
	case entity.MonsterBoss10:
		return []string{
			`  __|__  `,
			` /|o o|\ `,
			` | === | `,
			` |\___/| `,
			`  |___| `,
		}
	case entity.MonsterBoss15:
		return []string{
			` ,=====. `,
			`/ \   / \`,
			`|  (O)  |`,
			` \ |=| / `,
			`  '===' `,
		}
	case entity.MonsterBoss20:
		return []string{
			` ___/\___ `,
			`|  x  x  |`,
			`|  ===   |`,
			`|\_____/|`,
			`|_|   |_|`,
		}
	default:
		return []string{` ??? `}
	}
}
