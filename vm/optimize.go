package vm

import "math"

//
// Detects a previous drill, and uses rapid move to the previous known depth
//
func (vm *Machine) OptimizeDrills() {
	var (
		lastx, lasty, lastz float64
		npos                []Position = make([]Position, 0)
	)

	// TODO Reduce search to only drill moves
	fastDrill := func(index int, pos Position) (Position, Position, bool) {
		var depth float64
		var found bool
		for idx, m := range vm.posStack {
			if idx == index {
				break
			}
			if m.x == pos.x && m.y == pos.y {
				if m.z < depth {
					depth = m.z
					found = true
				}
			}
		}

		if found {
			if pos.z >= depth { // We have drilled all of it, so just modify old object
				pos.state.moveMode = rapidMoveMode
				return pos, pos, false
			} else {
				p := pos
				p.z = depth
				p.state.moveMode = rapidMoveMode
				return p, pos, true
			}
		} else {
			return pos, pos, false
		}
	}

	for idx, m := range vm.posStack {
		if m.x == lastx && m.y == lasty && m.z < lastz && m.state.moveMode == linearMoveMode {
			posn, poso, shouldinsert := fastDrill(idx, m)
			if shouldinsert {
				npos = append(npos, posn)
			}
			npos = append(npos, poso)
		} else {
			npos = append(npos, m)
		}
		lastx, lasty, lastz = m.x, m.y, m.z
	}
	vm.posStack = npos
}

//
// Uses rapid move for all Z-up only moves
//
func (vm *Machine) OptimizeLifts() {
	var lastx, lasty, lastz float64
	for idx, m := range vm.posStack {
		if m.x == lastx && m.y == lasty && m.z > lastz {
			// We got a lift! Let's make it faster, shall we?
			vm.posStack[idx].state.moveMode = rapidMoveMode
		}
		lastx, lasty, lastz = m.x, m.y, m.z
	}
}

//
// Kills redundant partial moves
//
func (vm *Machine) OptimizeMoves() {
	var (
		xstate, ystate, zstate       float64
		vecX, vecY, vecZ             float64
		lastvecX, lastvecY, lastvecZ float64
		npos                         []Position = make([]Position, 0)
	)

	for _, m := range vm.posStack {
		dx, dy, dz := m.x-xstate, m.y-ystate, m.z-zstate
		xstate, ystate, zstate = m.x, m.y, m.z

		if m.state.moveMode != rapidMoveMode && m.state.moveMode != linearMoveMode {
			// I'm not mentally ready for arc optimization yet...
			npos = append(npos, m)
			continue
		}

		if dx == 0 && dz == 0 && dy == 0 {
			// Why are we doing this again?!
			continue
		}

		norm := math.Sqrt(math.Pow(dx, 2) + math.Pow(dy, 2) + math.Pow(dz, 2))
		vecX, vecY, vecZ = dx/norm, dy/norm, dz/norm

		if lastvecX == vecX && lastvecY == vecY && lastvecZ == vecZ {
			npos[len(npos)-1] = m
		} else {
			npos = append(npos, m)
			lastvecX, lastvecY, lastvecZ = vecX, vecY, vecZ
		}
	}
	vm.posStack = npos
}
