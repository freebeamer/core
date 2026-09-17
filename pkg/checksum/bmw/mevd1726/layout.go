package mevd1726

type block struct {
	fileStart   uint64
	size        uint64
	identifier  uint32
	memoryStart uint32
	memoryEnd   uint32
	structures  []structure
}

type structure struct {
	offset    uint64
	algorithm byte
	start     uint32
	end       uint32
	seed      uint32
	expected  uint32
}

type manifestEntry struct {
	start      uint64
	size       uint64
	identifier uint32
	structures int
}

var validatedManifest = []manifestEntry{
	{start: 0, size: 0xfc04, identifier: 0x30, structures: 2},
	{start: 0x14000, size: 0x3f00, identifier: 0x00800020, structures: 1},
	{start: 0x18000, size: 0x7f00, identifier: 0x10, structures: 2},
	{start: 0x20000, size: 0x15fc04, identifier: 0x40, structures: 3},
	{start: 0x180000, size: 0x7fc04, identifier: 0x60, structures: 2},
	{start: 0x220000, size: 0x1dfc04, identifier: 0x50, structures: 1},
}
