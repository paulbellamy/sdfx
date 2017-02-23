//-----------------------------------------------------------------------------
/*

2D Signed Distance Functions

*/
//-----------------------------------------------------------------------------

package sdf

import "math"

//-----------------------------------------------------------------------------

type SDF2 interface {
	Evaluate(p V2) float64
	BoundingBox() Box2
}

//-----------------------------------------------------------------------------
// Basic SDF Functions

func sdf_box2d(p, s V2) float64 {
	p = p.Abs()
	d := p.Sub(s)
	k := s.Y - s.X
	if d.X > 0 && d.Y > 0 {
		return d.Length()
	}
	if p.Y-p.X > k {
		return d.Y
	}
	return d.X
}

//-----------------------------------------------------------------------------
// 2D Circle

type CircleSDF2 struct {
	radius float64
	bb     Box2
}

func Circle2D(radius float64) SDF2 {
	s := CircleSDF2{}
	s.radius = radius
	d := V2{radius, radius}
	s.bb = Box2{d.Negate(), d}
	return &s
}

func (s *CircleSDF2) Evaluate(p V2) float64 {
	return p.Length() - s.radius
}

func (s *CircleSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

// Multiple Circles
type MultiCircleSDF2 struct {
	radius    float64
	positions V2Set
	bb        Box2
}

// Return an SDF2 for multiple circles.
func NewMultiCircleSDF2(radius float64, positions V2Set) SDF2 {
	s := MultiCircleSDF2{}
	s.radius = radius
	s.positions = positions
	// work out the bounding box
	pmin := positions.Min().Sub(V2{radius, radius})
	pmax := positions.Max().Add(V2{radius, radius})
	s.bb = Box2{pmin, pmax}
	return &s
}

// Return the minimum distance to multiple circles.
func (s *MultiCircleSDF2) Evaluate(p V2) float64 {
	d := math.MaxFloat64
	for _, posn := range s.positions {
		d = Min(d, p.Sub(posn).Length()-s.radius)
	}
	return d
}

// Return the bounding box for multiple circles.
func (s *MultiCircleSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------
// 2D Box (rounded corners with round > 0)

type BoxSDF2 struct {
	size  V2
	round float64
	bb    Box2
}

func NewBoxSDF2(size V2, round float64) SDF2 {
	size = size.MulScalar(0.5)
	s := BoxSDF2{}
	s.size = size.SubScalar(round)
	s.round = round
	s.bb = Box2{size.Negate(), size}
	return &s
}

func (s *BoxSDF2) Evaluate(p V2) float64 {
	return sdf_box2d(p, s.size) - s.round
}

func (s *BoxSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------
// 2D Line

type LineSDF2 struct {
	l     float64 // line length
	round float64 // rounding
	bb    Box2    // bounding box
}

// Return a line from (-l/2,0) to (l/2,0)
func NewLineSDF2(l, round float64) SDF2 {
	s := LineSDF2{}
	s.l = l / 2
	s.round = round
	s.bb = Box2{V2{-s.l - round, -round}, V2{s.l + round, round}}
	return &s
}

// Return the minimum distance to the line
func (s *LineSDF2) Evaluate(p V2) float64 {
	p = p.Abs()
	if p.X <= s.l {
		return p.Y - s.round
	}
	return p.Sub(V2{s.l, 0}).Length() - s.round
}

// Return the bounding box for the line
func (s *LineSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

type OffsetSDF2 struct {
	sdf    SDF2
	offset float64
	bb     Box2
}

// Offset an SDF2 - add a constant to the distance function
func NewOffsetSDF2(sdf SDF2, offset float64) SDF2 {
	s := OffsetSDF2{}
	s.sdf = sdf
	s.offset = offset
	// work out the bounding box
	bb := sdf.BoundingBox()
	s.bb = NewBox2(bb.Center(), bb.Size().AddScalar(2*offset))
	return &s
}

func (s *OffsetSDF2) Evaluate(p V2) float64 {
	return s.sdf.Evaluate(p) - s.offset
}

func (s *OffsetSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------
// Cut an SDF2 along a line

type CutSDF2 struct {
	sdf SDF2
	a   V2   // point on line
	n   V2   // normal to line
	bb  Box2 // bounding box
}

// Cut the SDF2 along a line from a in direction v.
// The SDF2 to the right of the line remains.
func NewCutSDF2(sdf SDF2, a, v V2) SDF2 {
	s := CutSDF2{}
	s.sdf = sdf
	s.a = a
	v = v.Normalize()
	s.n = V2{-v.Y, v.X}
	// TODO - cut the bounding box
	s.bb = sdf.BoundingBox()
	return &s
}

func (s *CutSDF2) Evaluate(p V2) float64 {
	return Max(p.Sub(s.a).Dot(s.n), s.sdf.Evaluate(p))
}

func (s *CutSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------
// 2D Polygon

type PolySDF2 struct {
	vertex []V2      // vertices
	vector []V2      // unit line vectors
	length []float64 // line lengths
	bb     Box2      // bounding box
}

func NewPolySDF2(vertex []V2) SDF2 {
	s := PolySDF2{}

	n := len(vertex)
	if n < 3 {
		return nil
	}

	// Close the loop (if necessary)
	s.vertex = vertex
	if !vertex[0].Equals(vertex[n-1], 0) {
		s.vertex = append(s.vertex, vertex[0])
	}

	// allocate pre-calculated line segment info
	nsegs := len(s.vertex) - 1
	s.vector = make([]V2, nsegs)
	s.length = make([]float64, nsegs)

	vmin := s.vertex[0]
	vmax := s.vertex[0]

	for i := 0; i < nsegs; i++ {
		l := s.vertex[i+1].Sub(s.vertex[i])
		s.length[i] = l.Length()
		s.vector[i] = l.Normalize()
		vmin = vmin.Min(s.vertex[i])
		vmax = vmax.Max(s.vertex[i])
	}

	s.bb = Box2{vmin, vmax}
	return &s
}

func (s *PolySDF2) Evaluate(p V2) float64 {
	dd := math.MaxFloat64 // d^2 to polygon (>0)
	wn := 0               // winding number (inside/outside)

	// iterate over the line segments
	nsegs := len(s.vertex) - 1
	pb := p.Sub(s.vertex[0])

	for i := 0; i < nsegs; i++ {
		a := s.vertex[i]
		b := s.vertex[i+1]

		pa := pb
		pb = p.Sub(b)

		t := pa.Dot(s.vector[i])                        // t-parameter of projection onto line
		dn := pa.Dot(V2{s.vector[i].Y, -s.vector[i].X}) // normal distance from p to line

		// Distance to line segment
		if t < 0 {
			dd = Min(dd, pa.Length2()) // distance to vertex[0] of line
		} else if t > s.length[i] {
			dd = Min(dd, pb.Length2()) // distance to vertex[1] of line
		} else {
			dd = Min(dd, dn*dn) // normal distance to line
		}

		// Is the point in the polygon?
		// See: http://geomalgorithms.com/a03-_inclusion.html
		if a.Y <= p.Y {
			if b.Y > p.Y { // upward crossing
				if dn < 0 { // p is to the left of the line segment
					wn += 1 // up intersect
				}
			}
		} else {
			if b.Y <= p.Y { // downward crossing
				if dn > 0 { // p is to the right of the line segment
					wn -= 1 // down intersect
				}
			}
		}
	}

	// normalise d*d to d
	d := math.Sqrt(dd)
	if wn != 0 {
		// p is inside the polygon
		return -d
	}
	return d
}

func (s *PolySDF2) BoundingBox() Box2 {
	return s.bb
}

func (s *PolySDF2) Vertices() []V2 {
	return s.vertex
}

//-----------------------------------------------------------------------------
// Transform SDF2

type TransformSDF2 struct {
	sdf     SDF2
	matrix  M33
	inverse M33
	bb      Box2
}

func NewTransformSDF2(sdf SDF2, matrix M33) SDF2 {
	s := TransformSDF2{}
	s.sdf = sdf
	s.matrix = matrix
	s.inverse = matrix.Inverse()
	s.bb = matrix.MulBox(sdf.BoundingBox())
	return &s
}

func (s *TransformSDF2) Evaluate(p V2) float64 {
	q := s.inverse.MulPosition(p)
	return s.sdf.Evaluate(q)
}

func (s *TransformSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------
// ArraySDF2: Create an X by Y array of a given SDF2
// num = the array size
// size = the step size

type ArraySDF2 struct {
	sdf  SDF2
	num  V2i
	step V2
	min  MinFunc
	k    float64
	bb   Box2
}

func NewArraySDF2(sdf SDF2, num V2i, step V2) SDF2 {
	// check the number of steps
	if num[0] <= 0 || num[1] <= 0 {
		return nil
	}
	s := ArraySDF2{}
	s.sdf = sdf
	s.num = num
	s.step = step
	s.min = NormalMin
	// work out the bounding box
	bb0 := sdf.BoundingBox()
	bb1 := bb0.Translate(step.Mul(num.SubScalar(1).ToV2()))
	s.bb = bb0.Extend(bb1)
	return &s
}

// set the minimum function to control blending
func (s *ArraySDF2) SetMin(min MinFunc, k float64) {
	s.min = min
	s.k = k
}

func (s *ArraySDF2) Evaluate(p V2) float64 {
	d := math.MaxFloat64
	for j := 0; j < s.num[0]; j++ {
		for k := 0; k < s.num[1]; k++ {
			x := p.Sub(V2{float64(j) * s.step.X, float64(k) * s.step.Y})
			d = s.min(d, s.sdf.Evaluate(x), s.k)
		}
	}
	return d
}

func (s *ArraySDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

type RotateSDF2 struct {
	sdf  SDF2
	num  int
	step M33
	min  MinFunc
	k    float64
	bb   Box2
}

func NewRotateSDF2(sdf SDF2, num int, step M33) SDF2 {
	// check the number of steps
	if num <= 0 {
		return nil
	}
	s := RotateSDF2{}
	s.sdf = sdf
	s.num = num
	s.step = step.Inverse()
	s.min = NormalMin
	// work out the bounding box
	v := sdf.BoundingBox().Vertices()
	bb_min := v[0]
	bb_max := v[0]
	for i := 0; i < s.num; i++ {
		bb_min = bb_min.Min(v.Min())
		bb_max = bb_max.Max(v.Max())
		v.MulVertices(step)
	}
	s.bb = Box2{bb_min, bb_max}
	return &s
}

// Return the minimum distance to the object.
func (s *RotateSDF2) Evaluate(p V2) float64 {
	d := math.MaxFloat64
	rot := Identity2d()
	for i := 0; i < s.num; i++ {
		x := rot.MulPosition(p)
		d = s.min(d, s.sdf.Evaluate(x), s.k)
		rot = rot.Mul(s.step)
	}
	return d
}

// Set the minimum function to control blending.
func (s *RotateSDF2) SetMin(min MinFunc, k float64) {
	s.min = min
	s.k = k
}

// Return the bounding box.
func (s *RotateSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

type RotateCopySDF2 struct {
	sdf   SDF2
	theta float64
	bb    Box2
}

// Rotate and copy an SDF2 TAU radians about the origin.
// sdf = SDF2 to rotate and copy
// num = numer of copies
func NewRotateCopySDF2(sdf SDF2, num int) SDF2 {
	// check the number of steps
	if num <= 0 {
		return nil
	}
	s := RotateCopySDF2{}
	s.sdf = sdf
	s.theta = TAU / float64(num)
	// work out the bounding box
	r_max := sdf.BoundingBox().Max.Length()
	s.bb = Box2{V2{-r_max, -r_max}, V2{r_max, r_max}}
	return &s
}

// Return the minimum distance to the object.
func (s *RotateCopySDF2) Evaluate(p V2) float64 {
	// Map p to a point in the first copy sector.
	pnew := PolarToXY(p.Length(), SawTooth(math.Atan2(p.Y, p.X), s.theta))
	return s.sdf.Evaluate(pnew)
}

// Return the bounding box.
func (s *RotateCopySDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

type SliceSDF2 struct {
	sdf SDF3 // the sdf3 being sliced
	a   V3   // 3d point for 2d origin
	u   V3   // vector for the 2d x-axis
	v   V3   // vector for the 2d y-axis
	bb  Box2 // bounding box
}

// Create an SDF2 from a plane sliced through an SDF3.
// sdf = SDF3 to be sliced
// a = point on plane
// n = normal to plane
func Slice2D(sdf SDF3, a, n V3) SDF2 {
	s := SliceSDF2{}
	s.sdf = sdf
	s.a = a
	// work out the x/y vectors on the plane.
	if n.X == 0 {
		s.u = V3{1, 0, 0}
	} else if n.Y == 0 {
		s.u = V3{0, 1, 0}
	} else if n.Z == 0 {
		s.u = V3{0, 0, 1}
	} else {
		s.u = V3{n.Y, -n.X, 0}
	}
	s.v = n.Cross(s.u)
	s.u = s.u.Normalize()
	s.v = s.v.Normalize()
	// work out the bounding box
	// TODO: This is bigger than it needs to be. We could consider intersection
	// between the plane and the edges of the 3d bounding box for a smaller 2d
	// bounding box in some circumstances.
	v3 := sdf.BoundingBox().Vertices()
	v2 := make(V2Set, len(v3))
	n = n.Normalize()
	for i, v := range v3 {
		// project the 3d bounding box vertex onto the plane
		va := v.Sub(s.a)
		pa := va.Sub(n.MulScalar(n.Dot(va)))
		// work out the 3d point in terms of the 2d unit vectors
		v2[i] = V2{pa.Dot(s.u), pa.Dot(s.v)}
	}
	s.bb = Box2{v2.Min(), v2.Max()}
	return &s
}

// Return the minimum distance to the object.
func (s *SliceSDF2) Evaluate(p V2) float64 {
	pnew := s.a.Add(s.u.MulScalar(p.X)).Add(s.v.MulScalar(p.Y))
	return s.sdf.Evaluate(pnew)
}

// Return the bounding box.
func (s *SliceSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

type UnionSDF2 struct {
	s0  SDF2
	s1  SDF2
	min MinFunc
	k   float64
	bb  Box2
}

func NewUnionSDF2(s0, s1 SDF2) SDF2 {
	if s0 == nil {
		return s1
	}
	if s1 == nil {
		return s0
	}
	s := UnionSDF2{}
	s.s0 = s0
	s.s1 = s1
	s.min = NormalMin
	s.bb = s0.BoundingBox().Extend(s1.BoundingBox())
	return &s
}

// set the minimum function to control blending
func (s *UnionSDF2) SetMin(min MinFunc, k float64) {
	s.min = min
	s.k = k
}

func (s *UnionSDF2) Evaluate(p V2) float64 {
	a := s.s0.Evaluate(p)
	b := s.s1.Evaluate(p)
	return s.min(a, b, s.k)
}

func (s *UnionSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------

// Difference of SDF2s
type DifferenceSDF2 struct {
	s0  SDF2
	s1  SDF2
	max MaxFunc
	k   float64
	bb  Box2
}

// Return the difference of two SDF2 objects, s0 - s1.
func NewDifferenceSDF2(s0, s1 SDF2) SDF2 {
	if s1 == nil {
		return s0
	}
	if s0 == nil {
		return nil
	}
	s := DifferenceSDF2{}
	s.s0 = s0
	s.s1 = s1
	s.max = NormalMax
	s.bb = s0.BoundingBox()
	return &s
}

// Return the minimum distance to the object.
func (s *DifferenceSDF2) Evaluate(p V2) float64 {
	return s.max(s.s0.Evaluate(p), -s.s1.Evaluate(p), s.k)
}

// Set the maximum function to control blending.
func (s *DifferenceSDF2) SetMax(max MaxFunc, k float64) {
	s.max = max
	s.k = k
}

// Return the bounding box.
func (s *DifferenceSDF2) BoundingBox() Box2 {
	return s.bb
}

//-----------------------------------------------------------------------------
