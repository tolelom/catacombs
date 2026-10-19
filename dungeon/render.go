package dungeon

import "fmt"

type FloorTheme struct {
	WallColor  string
	FloorColor string
	Name       string
}

func GetFloorTheme(floorNum int) FloorTheme {
	switch {
	case floorNum <= 5:
		return FloorTheme{"90", "245", "Stone Halls"}
	case floorNum <= 10:
		return FloorTheme{"22", "28", "Mossy Caverns"}
	case floorNum <= 15:
		return FloorTheme{"88", "202", "Lava Depths"}
	default:
		return FloorTheme{"53", "129", "Shadow Realm"}
	}
}

const (
	ansiReset     = "\033[0m"
	ansiBright    = "\033[1m"
	ansiDim       = "\033[2m"
	ansiFgWhite   = "\033[97m"
	ansiFgGray    = "\033[90m"
	ansiFgGreen   = "\033[92m"
	ansiFgRed     = "\033[91m"
	ansiFgBrRed   = "\033[1;91m"
	ansiFgYellow  = "\033[93m"
	ansiFgCyan    = "\033[96m"
	ansiFgMagenta = "\033[95m"
)

func roomOwnership(floor *Floor) [][]int {
	owner := make([][]int, floor.Height)
	for y := 0; y < floor.Height; y++ {
		owner[y] = make([]int, floor.Width)
		for x := 0; x < floor.Width; x++ {
			owner[y][x] = -1
		}
	}
	for i, room := range floor.Rooms {
		for dy := 0; dy < room.H; dy++ {
			for dx := 0; dx < room.W; dx++ {
				ty := room.Y + dy
				tx := room.X + dx
				if ty >= 0 && ty < floor.Height && tx >= 0 && tx < floor.Width {
					owner[ty][tx] = i
				}
			}
		}
	}
	return owner
}

func buildCorridorVisibility(floor *Floor, owner [][]int) [][]Visibility {
	vis := make([][]Visibility, floor.Height)
	for y := 0; y < floor.Height; y++ {
		vis[y] = make([]Visibility, floor.Width)
	}

	type pos struct{ y, x int }
	queue := []pos{}

	for y := 0; y < floor.Height; y++ {
		for x := 0; x < floor.Width; x++ {
			if floor.Tiles[y][x] != TileCorridor {
				continue
			}
			best := Hidden
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					ny, nx := y+dy, x+dx
					if ny >= 0 && ny < floor.Height && nx >= 0 && nx < floor.Width {
						ri := owner[ny][nx]
						if ri >= 0 {
							v := GetRoomVisibility(floor, ri)
							if v > best {
								best = v
							}
						}
					}
				}
			}
			if best > Hidden {
				vis[y][x] = best
				queue = append(queue, pos{y, x})
			}
		}
	}

	dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			ny, nx := cur.y+d[0], cur.x+d[1]
			if ny >= 0 && ny < floor.Height && nx >= 0 && nx < floor.Width {
				if floor.Tiles[ny][nx] == TileCorridor && vis[ny][nx] < vis[cur.y][cur.x] {
					vis[ny][nx] = vis[cur.y][cur.x]
					queue = append(queue, pos{ny, nx})
				}
			}
		}
	}

	return vis
}

func wallVisible(floor *Floor, owner [][]int, corrVis [][]Visibility, x, y int) Visibility {
	best := Hidden
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			ny, nx := y+dy, x+dx
			if ny >= 0 && ny < floor.Height && nx >= 0 && nx < floor.Width {
				if floor.Tiles[ny][nx] == TileFloor {
					ri := owner[ny][nx]
					if ri >= 0 {
						v := GetRoomVisibility(floor, ri)
						if v > best {
							best = v
						}
					}
				}
				if floor.Tiles[ny][nx] == TileCorridor {
					if corrVis[ny][nx] > best {
						best = corrVis[ny][nx]
					}
				}
			}
		}
	}
	return best
}

func RenderFloor(floor *Floor, currentRoom int, showFog bool) string {
	if floor == nil || floor.Tiles == nil {
		return ""
	}
	theme := GetFloorTheme(floor.Number)

	owner := roomOwnership(floor)

	type marker struct {
		symbol string
		color  string
	}
	markers := make(map[[2]int]marker)

	for i, room := range floor.Rooms {
		cx := room.X + room.W/2
		cy := room.Y + room.H/2
		vis := GetRoomVisibility(floor, i)
		showRoom := !showFog || vis != Hidden
		if !showRoom && currentRoom >= 0 && currentRoom < len(floor.Rooms) {
			for _, n := range floor.Rooms[currentRoom].Neighbors {
				if n == i {
					showRoom = true
					break
				}
			}
		}
		if showRoom {
			sym, col := roomMarker(room, i, i == currentRoom)
			markers[[2]int{cy, cx}] = marker{sym, col}
		}
	}

	var playerPos [2]int
	if currentRoom >= 0 && currentRoom < len(floor.Rooms) {
		r := floor.Rooms[currentRoom]
		playerPos = [2]int{r.Y + r.H/2, r.X + r.W/2}
	}

	corrVis := buildCorridorVisibility(floor, owner)

	buf := make([]byte, 0, floor.Width*floor.Height*4)

	for y := 0; y < floor.Height; y++ {
		for x := 0; x < floor.Width; x++ {
			tile := floor.Tiles[y][x]

			var vis Visibility
			if showFog {
				switch tile {
				case TileFloor:
					ri := owner[y][x]
					if ri >= 0 {
						vis = GetRoomVisibility(floor, ri)
					} else {
						vis = Hidden
					}
				case TileCorridor:
					vis = corrVis[y][x]
				case TileWall:
					vis = wallVisible(floor, owner, corrVis, x, y)
				default:
					vis = Hidden
				}
			} else {
				vis = Visible
			}

			if vis == Hidden {
				buf = append(buf, ' ')
				continue
			}

			if y == playerPos[0] && x == playerPos[1] {
				buf = append(buf, []byte(fmt.Sprintf("%s%s@%s", ansiBright, ansiFgGreen, ansiReset))...)
				continue
			}

			if m, ok := markers[[2]int{y, x}]; ok {
				if vis == Visible {
					buf = append(buf, []byte(fmt.Sprintf("%s%s%s", m.color, m.symbol, ansiReset))...)
				} else {
					buf = append(buf, []byte(fmt.Sprintf("%s%s%s", ansiFgGray, m.symbol, ansiReset))...)
				}
				continue
			}

			var ch byte
			switch tile {
			case TileWall:
				ch = '#'
			case TileFloor:
				ch = '.'
			case TileCorridor:
				ch = '+'
			case TileDoor:
				ch = '/'
			default:
				ch = ' '
			}

			if vis == Visible {
				switch tile {
				case TileWall:
					buf = append(buf, []byte(fmt.Sprintf("\033[38;5;%sm%c\033[0m", theme.WallColor, ch))...)
				case TileFloor:
					buf = append(buf, []byte(fmt.Sprintf("\033[38;5;%sm%c\033[0m", theme.FloorColor, ch))...)
				default:
					buf = append(buf, []byte(fmt.Sprintf("%s%s%c%s", ansiBright, ansiFgWhite, ch, ansiReset))...)
				}
			} else {
				buf = append(buf, []byte(fmt.Sprintf("%s%c%s", ansiFgGray, ch, ansiReset))...)
			}
		}
		buf = append(buf, '\n')
	}

	return string(buf)
}

func roomMarker(room *Room, roomIdx int, isCurrent bool) (string, string) {
	num := fmt.Sprintf("%d", roomIdx)
	if room.Cleared {
		return num, ansiFgGray
	}
	switch room.Type {
	case RoomCombat:
		return num, ansiFgRed
	case RoomTreasure:
		return num, ansiFgYellow
	case RoomShop:
		return num, ansiFgCyan
	case RoomEvent:
		return num, ansiFgMagenta
	case RoomBoss:
		return num, ansiFgBrRed
	case RoomEmpty:
		return num, ansiFgGray
	case RoomMimic:
		return num, ansiFgYellow
	case RoomGamble:
		return num, ansiFgYellow
	case RoomCursedAltar:
		return num, ansiFgMagenta
	case RoomWanderingMerchant:
		return num, ansiFgCyan
	default:
		return num, ansiReset
	}
}
