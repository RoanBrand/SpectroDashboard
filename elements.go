package main

// Database to actual.
var elementMap = map[string]string{
	"0x00000001-C":  "C",
	"0x00000003-Si": "Si",
	"0x00000005-Mn": "Mn",
	"0x00000007-P":  "P",
	"0x00000009-S":  "S",
	"0x00000019-Cu": "Cu",
	"0x0000000B-Cr": "Cr",
	"0x00000015-Al": "Al",
	"0x0000001F-Ti": "Ti",
	"0x00000027-Sn": "Sn",
	"0x00000031-Zn": "Zn",
	"0x00000025-Pb": "Pb",
}

// Elements to display, and their order.
var elementOrder = map[string]int{
	"C":  0,
	"Si": 1,
	"Mn": 2,
	"P":  3,
	"S":  4,
	"Cu": 5,
	"Cr": 6,
	"Al": 7,
	"Ti": 8,
	"Sn": 9,
	"Zn": 10,
	"Pb": 11,
}
