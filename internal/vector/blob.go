package vector

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Float32ToBlob encodes little-endian float32 vector.
func Float32ToBlob(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// BlobToFloat32 decodes little-endian float32 vector of expected dimension.
func BlobToFloat32(b []byte, dim int) ([]float32, error) {
	if dim <= 0 {
		return nil, fmt.Errorf("invalid dim %d", dim)
	}
	if len(b) != dim*4 {
		return nil, fmt.Errorf("blob length %d != dim*4 %d", len(b), dim*4)
	}
	out := make([]float32, dim)
	for i := 0; i < dim; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out, nil
}

// CosineDistance returns 1 - cosine_similarity (0 = identical).
func CosineDistance(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return math.MaxFloat64
	}
	var dot, na, nb float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na < 1e-12 || nb < 1e-12 {
		return 1
	}
	sim := dot / (math.Sqrt(na) * math.Sqrt(nb))
	// clamp
	if sim > 1 {
		sim = 1
	}
	if sim < -1 {
		sim = -1
	}
	return 1 - sim
}
