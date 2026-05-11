// practices: [resilience-patterns, sre]
package main

import "errors"

// errLockWouldBlock is returned by flockLockNB when the lock is already held
// by another process. Platform implementations map their OS-specific
// "would block" errors (EWOULDBLOCK, EAGAIN, ERROR_LOCK_VIOLATION) to this.
var errLockWouldBlock = errors.New("file lock already held by another process")
