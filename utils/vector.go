package utils

import "math"
import "fmt"

type Vector struct {
	X, Y, Z float64
}

func (v Vector) Dot(o Vector) float64 {
	return v.X*o.X + v.Y*o.Y + v.Z*o.Z
}

func (v Vector) Cross(o Vector) Vector {
	return Vector{
		X: v.Y*o.Z - v.Z*o.Y,
		Y: v.Z*o.X - v.X*o.Z,
		Z: v.X*o.Y - v.Y*o.X,
	}
}

func (v Vector) Norm() float64 {
	return math.Sqrt(v.Dot(v))
}

func (v Vector) Sum(o Vector) Vector {
	return Vector{
		X: v.X + o.X,
		Y: v.Y + o.Y,
		Z: v.Z + o.Z,
	}
}

func (v Vector) Diff(o Vector) Vector {
	return Vector{
		X: v.X - o.X,
		Y: v.Y - o.Y,
		Z: v.Z - o.Z,
	}
}

func (v Vector) Divide(d float64) Vector {
	return Vector{
		X: v.X / d,
		Y: v.Y / d,
		Z: v.Z / d,
	}
}

func (v Vector) String() string {
	return fmt.Sprintf("Vector{X: %f, Y: %f, Z: %f}", v.X, v.Y, v.Z)
}
