package vm

import "github.com/joushou/gocnc/vector"

type CoordinateSystem struct {
	coordinateSystems       []vector.Vector
	offset                  vector.Vector
	offsetEnabled           bool
	currentCoordinateSystem int
	override                bool
}

func (c *CoordinateSystem) expandIfNecessary(s int) {
	for len(c.coordinateSystems) <= s {
		c.coordinateSystems = append(c.coordinateSystems, vector.Vector{})
	}
}

func (c *CoordinateSystem) SelectCoordinateSystem(s int) {
	c.expandIfNecessary(s)
	c.currentCoordinateSystem = s
}

func (c *CoordinateSystem) SetCoordinateSystem(x, y, z float64, s int) {
	c.expandIfNecessary(s)
	c.coordinateSystems[s] = vector.Vector{x, y, z}
}

func (c *CoordinateSystem) SetOffset(x, y, z float64) {
	c.offset.X = x
	c.offset.Y = y
	c.offset.Z = z
}

func (c *CoordinateSystem) EnableOffset() {
	c.offsetEnabled = true
}

func (c *CoordinateSystem) DisableOffset() {
	c.offsetEnabled = false
}

func (c *CoordinateSystem) EraseOffset() {
	c.offset = vector.Vector{}
	c.offsetEnabled = false
}

func (c *CoordinateSystem) GetCoordinateSystem() vector.Vector {
	c.expandIfNecessary(c.currentCoordinateSystem)
	if c.override {
		return vector.Vector{}
	}
	v := c.coordinateSystems[c.currentCoordinateSystem]
	if c.offsetEnabled {
		v = v.Sum(c.offset)
	}
	return v
}

func (c *CoordinateSystem) ApplyCoordinateSystem(x, y, z float64) (float64, float64, float64) {
	if c.override {
		return x, y, z
	}

	c.expandIfNecessary(c.currentCoordinateSystem)
	v := c.coordinateSystems[c.currentCoordinateSystem]

	if c.offsetEnabled {
		v = v.Sum(c.offset)
	}

	x += v.X
	y += v.Y
	z += v.Z

	return x, y, z
}

func (c *CoordinateSystem) Override() {
	c.override = true
}

func (c *CoordinateSystem) CancelOverride() {
	c.override = false
}

func (c *CoordinateSystem) OverrideActive() bool {
	return c.override
}

func (c *CoordinateSystem) OffsetActive() bool {
	return c.offsetEnabled
}
