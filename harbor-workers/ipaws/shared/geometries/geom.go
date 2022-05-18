package geometries

import (
	"fmt"
	"github.com/golang/geo/s2"
	"math"
	"strconv"
	"strings"
)

const degConv = float64(180) / math.Pi
const radConv = math.Pi / float64(180)
const latScalar = float64(110.574)
const longScalar = float64(88.5959965)

type BBRect struct {
	LatLo float64
	LatHi float64
	LngLo float64
	LngHi float64

	Polygon *s2.Loop
}

func (b *BBRect) ScaleBoundingBox(scaleFactorKm float64) {
	scaleLatKmToDegrees := scaleFactorKm / latScalar
	b.LatLo = b.LatLo - scaleLatKmToDegrees
	b.LatHi = b.LatHi + scaleLatKmToDegrees
	// go with larger lat scale
	scaleLongKmToDegrees := scaleFactorKm / (math.Cos(b.LatLo*radConv) * longScalar)
	b.LngLo = b.LngLo - scaleLongKmToDegrees
	b.LngHi = b.LngHi + scaleLongKmToDegrees
}

func (b *BBRect) containsPoint(lat float64, long float64) bool {
	tll := s2.LatLngFromDegrees(lat, long)
	tpt := s2.PointFromLatLng(tll)

	return b.Polygon.ContainsPoint(tpt)
}

func GetBoundingBoxFromPolygonString(vertices string) (*BBRect, error) {
	points, err := getVerticesFromPolygonString(vertices)
	if err != nil {
		return nil, fmt.Errorf("error getting vertices from string: %s", err)
	}

	// ipaws polygon data usually comes in backwards, but sometimes
	// it does not. sometimes the loop is degenerate and not
	// closed. this is a safeguard against these cases.
	// setting the radius to be of the unit sphere,
	// the area of the globe is ~ 4 * pi. the US is 1.8%.
	// anything over 0.1 would be nonsense, inverting
	// the loop will either give the correct area or
	// in the case of a non-closed loop 0.0
	loop := s2.LoopFromPoints(*points)
	if loop.Area() > 0.1 {
		loop.Invert()
	}
	rect := loop.RectBound()

	ltl := rect.Lat.Lo * degConv
	lth := rect.Lat.Hi * degConv
	lnl := rect.Lng.Lo * degConv
	lnh := rect.Lng.Hi * degConv

	rr := BBRect{
		LatLo:   ltl,
		LatHi:   lth,
		LngLo:   lnl,
		LngHi:   lnh,
		Polygon: getBoundingBoxPolygonLoop(ltl, lth, lnl, lnh),
	}

	return &rr, nil
}

func GetBoundingBoxFromString(bounds string) (*BBRect, error) {
	ptSplit := strings.Split(bounds, " ")
	if len(ptSplit) <= 0 || ptSplit[0] == "" {
		return nil, nil
	}
	if len(ptSplit) != 4 {
		return nil, fmt.Errorf("boundingBox not in correct format, found %d points, req 4", len(ptSplit))
	}
	latL, _ := strconv.ParseFloat(ptSplit[0], 64)
	latH, _ := strconv.ParseFloat(ptSplit[1], 64)
	lngL, _ := strconv.ParseFloat(ptSplit[2], 64)
	lngH, _ := strconv.ParseFloat(ptSplit[3], 64)

	bbr := BBRect{
		LatLo:   latL,
		LatHi:   latH,
		LngLo:   lngL,
		LngHi:   lngH,
		Polygon: getBoundingBoxPolygonLoop(latL, latH, lngL, lngH),
	}

	return &bbr, nil
}

func getBoundingBoxPolygonLoop(latL float64, latH float64, lngL float64, lngH float64) *s2.Loop {
	points := []s2.Point {
		s2.PointFromLatLng(
			s2.LatLngFromDegrees(latL, lngL)),
		s2.PointFromLatLng(
			s2.LatLngFromDegrees(latL, lngH)),
		s2.PointFromLatLng(
			s2.LatLngFromDegrees(latH, lngH)),
		s2.PointFromLatLng(
			s2.LatLngFromDegrees(latH, lngL)),
	}

	loop := s2.LoopFromPoints(points)
	if loop.Area() > 0.1 {
		loop.Invert()
	}

	return loop
}

func getVerticesFromPolygonString(vertices string) (*[]s2.Point, error) {
	var points []s2.Point
	strVertexArr := strings.Split(vertices, " ")

	for _, strV := range strVertexArr {
		coords := strings.Split(strV, ",")

		if len(coords) != 2 {
			return nil, fmt.Errorf("WARNING: vertices poorly formed: %s", vertices)
		}
		latD, errLat := strconv.ParseFloat(coords[0], 64)
		lonD, errLng := strconv.ParseFloat(coords[1], 64)
		if errLat != nil || errLng != nil {
			return nil, fmt.Errorf("WARNING: vertices poorly formed: %s", vertices)
		}

		ll := s2.LatLngFromDegrees(latD, lonD)
		pt := s2.PointFromLatLng(ll)
		points = append(points, pt)
	}

	return &points, nil
}
