package main

import (
	"fmt"
	"github.com/golang/geo/s2"
	"strconv"
	"strings"
)

type BBRect struct {
	LatLo float64
	LatHi float64
	LngLo float64
	LngHi float64

	Polygon *s2.Loop
}

func (b *BBRect) ContainsPoint(lat float64, long float64) bool {
	tll := s2.LatLngFromDegrees(lat, long)
	tpt := s2.PointFromLatLng(tll)

	return b.Polygon.ContainsPoint(tpt)
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
		LatLo: latL,
		LatHi: latH,
		LngLo: lngL,
		LngHi: lngH,
		Polygon: GetBoundingBoxPolygonLoop(latL, latH, lngL, lngH),
	}

	return &bbr, nil
}

func GetBoundingBoxPolygonLoop(latL float64, latH float64, lngL float64, lngH float64) *s2.Loop {
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
	if loop.Area() >  0.1 {
		loop.Invert()
	}

	return loop
}
