package disk

import "time"

// nowFunc returns current time; replaced in tests for deterministic timing
var nowFunc = time.Now
