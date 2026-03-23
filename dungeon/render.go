package dungeon

import "fmt"

// ANSI color codes
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

// roomOwnership maps each tile coordinate to the room index that contains it (-1 = none).
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

// corridorVisibility determines if a corridor tile should be visible.
// A corridor is visible if it's adjacent to a visible or visited room.
func corridorVisible(floor *Floor, owner [][]int, x, y int) Visibility {
	best := Hidden
	// Check neighboring tiles for room ownership
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
	// Also check along the corridor path: if this corridor connects two rooms,
	// it should be visible if either room is visible/visited.
	// The adjacency check above handles most cases.
	return best
}

// wallVisibility determines if a wall tile should be shown based on adjacent rooms.
func wallVisible(floor *Floor, owner [][]int, x, y int) Visibility {
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
					cv := corridorVisible(floor, owner, nx, ny)
					if cv > best {
						best = cv
					}
				}
			}
		}
	}
	return best
}

// RenderFloor renders the tile map as a colored ASCII string.
func RenderFloor(floor *Floor, currentRoom int, showFog bool) string {
	if floor == nil || floor.Tiles == nil {
		return ""
	}

	owner := roomOwnership(floor)

	// Find room centers for content markers
	type marker struct {
		symbol string
		color  string
	}
	markers := make(map[[2]int]marker)

	for i, room := range floor.Rooms {
		cx := room.X + room.W/2
		cy := room.Y + room.H/2
		vis := GetRoomVisibility(floor, i)
		if !showFog || vis != Hidden {
			sym, col := roomMarker(room, i == currentRoom)
			markers[[2]int{cy, cx}] = marker{sym, col}
		}
	}

	// Player position at center of current room
	var playerPos [2]int
	if currentRoom >= 0 && currentRoom < len(floor.Rooms) {
		r := floor.Rooms[currentRoom]
		playerPos = [2]int{r.Y + r.H/2, r.X + r.W/2}
	}

	buf := make([]byte, 0, floor.Width*floor.Height*4)

	for y := 0; y < floor.Height; y++ {
		for x := 0; x < floor.Width; x++ {
			tile := floor.Tiles[y][x]

			// Determine visibility of this tile
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
					vis = corridorVisible(floor, owner, x, y)
				case TileWall:
					vis = wallVisible(floor, owner, x, y)
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

			// Check for player marker
			if y == playerPos[0] && x == playerPos[1] {
				buf = append(buf, []byte(fmt.Sprintf("%s%s@%s", ansiBright, ansiFgGreen, ansiReset))...)
				continue
			}

			// Check for room content marker
			if m, ok := markers[[2]int{y, x}]; ok {
				if vis == Visible {
					buf = append(buf, []byte(fmt.Sprintf("%s%s%s", m.color, m.symbol, ansiReset))...)
				} else {
					buf = append(buf, []byte(fmt.Sprintf("%s%s%s", ansiFgGray, m.symbol, ansiReset))...)
				}
				continue
			}

			// Render tile
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
				buf = append(buf, []byte(fmt.Sprintf("%s%s%c%s", ansiBright, ansiFgWhite, ch, ansiReset))...)
			} else {
				// Visited but not current — dim
				buf = append(buf, []byte(fmt.Sprintf("%s%c%s", ansiFgGray, ch, ansiReset))...)
			}
		}
		buf = append(buf, '\n')
	}

	return string(buf)
}

func roomMarker(room *Room, isCurrent bool) (string, string) {
	if room.Cleared {
		return ".", ansiFgGray
	}
	switch room.Type {
	case RoomCombat:
		return "D", ansiFgRed
	case RoomTreasure:
		return "$", ansiFgYellow
	case RoomShop:
		return "S", ansiFgCyan
	case RoomEvent:
		return "?", ansiFgMagenta
	case RoomBoss:
		return "B", ansiFgBrRed
	case RoomEmpty:
		return ".", ansiFgGray
	default:
		return " ", ansiReset
	}
}
