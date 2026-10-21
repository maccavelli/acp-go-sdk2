//go:build !unix && !windows

package agentproc

// platformChild has no modes of its own here.
func platformChild(string) (int, bool) { return 0, false }
