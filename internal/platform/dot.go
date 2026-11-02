package platform

// Mark is one agent dot - the label goes above the dot and both are drawn in Fill outlined with Ring
type Mark struct {
	X, Y       int // the dot's center in physical pixels
	Label      string
	Fill, Ring uint32 // 0xRRGGBB
}
