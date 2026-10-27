// Package acptest stands up an ACP connection pair for a test, records the
// frames that pass between the sides, and supplies a scripted agent and an
// in-memory client
// (docs/decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md,
// item 5).
//
// It uses only package acp's public constructors, so a pair sees what a
// program sees: the hold until initialize, the capability gates, and the
// close and wait behaviour. It is on encoding/json v1 and needs no
// dependency.
//
// Connect is the entry point: it joins an acp.Agent and an acp.Client over
// two pipes and closes and waits for both when the test ends, within a bound;
// ConnectRecorded records the client side's frames as it does so. Recorder
// copies the frames of a reader or a writer, each with the way it passed, in
// a canonical form for goldens, and ValidateGolden checks recorded frames
// against the schema's types. ScriptAgent plays scripted prompt turns, one
// that fails, reports usage or waits for a cancel among them; MemoryClient
// serves files from memory, answers permission requests and elicitations from
// a script, and serves terminals through a TerminalHost. Abs makes a fixture
// path absolute on the host, as the SDK requires of a path it receives
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 8).
package acptest
