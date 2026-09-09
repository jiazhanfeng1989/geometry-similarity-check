package similarity

import "testing"

func BenchmarkScoreLongDisjointLegs(b *testing.B) {
	reference := straightLine(37.0, -122.0, 20, 5000)
	candidate := offsetEast(reference, 5000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Score(candidate, reference); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScoreLongIdenticalLegs(b *testing.B) {
	reference := straightLine(37.0, -122.0, 20, 5000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Score(reference, reference); err != nil {
			b.Fatal(err)
		}
	}
}
