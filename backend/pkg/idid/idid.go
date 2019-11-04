package idid

import (
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type I [32]byte

func Pack(u uuid.I, l ulid.I) I {
	var p I
	copy(p[0:16], u[:])
	copy(p[16:32], l[:])
	return p
}

// returns `<uuid>0000...`
func RangeMin(u uuid.I) I {
	var min I
	copy(min[0:16], u[:])
	return min
}

// returns `<uuid>ffff...`
func RangeMax(u uuid.I) I {
	var max I
	copy(max[0:16], u[:])
	for i := 16; i < 32; i++ {
		max[i] = 0xff
	}
	return max
}

func RangeMinMax(u uuid.I) (I, I) {
	return RangeMin(u), RangeMax(u)
}
