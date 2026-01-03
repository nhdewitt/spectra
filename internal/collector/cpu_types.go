package collector

type CPURaw struct {
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	IOWait    uint64
	IRQ       uint64
	SoftIRQ   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
}

type CPUDelta struct {
	User      uint64
	Nice      uint64
	System    uint64
	Idle      uint64
	IOWait    uint64
	IRQ       uint64
	SoftIRQ   uint64
	Steal     uint64
	Guest     uint64
	GuestNice uint64
	Total     uint64 // Sum of all time
	Used      uint64 // Total - Idle - IOWait
}

type CPUDeltaWindows struct {
	User      float64
	Nice      float64
	System    float64
	Idle      float64
	IOWait    float64
	IRQ       float64
	SoftIRQ   float64
	Steal     float64
	Guest     float64
	GuestNice float64
	Total     float64 // Sum of all time
	Used      float64 // Total - Idle - IOWait
}
