package platform

// Setup reports how startup went so doctor can say what lowkey asked Windows for
type Setup struct {
	PriorityErr error
	EcoErr      error
}
