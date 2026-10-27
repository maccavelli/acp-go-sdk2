// Package clienthost holds ready implementations of the client methods an
// ACP client runs on the agent's behalf: Terminals runs the commands of the
// five terminal methods, and Files reads and writes text files for the two
// file-system methods (0041-MADR-close-the-gaps-the-consumers-wait-on.md,
// items 2 and 3).
//
// Each is a type a client embeds. Embedded beside Refusals, which refuses
// every optional client method with method-not-found one level deeper, its
// own methods win and the rest are refused:
//
//	type client struct {
//		clienthost.Terminals
//		clienthost.Files
//		clienthost.Refusals
//	}
//
// The client still implements SessionUpdate and RequestPermission, which
// neither provides, and advertises the capability of each method it serves
// in its initialize request: the SDK refuses a method whose capability the
// client did not advertise before the method runs.
package clienthost
