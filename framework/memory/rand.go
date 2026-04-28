package memory

import "crypto/rand"

// randReader is crypto/rand.Reader, aliased for testability.
var randReader = rand.Reader
