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
// two pipes and closes and waits for both when the test ends. Recorder copies
// the frames of a reader or a writer, in a canonical form for goldens.
// ScriptAgent plays a scripted prompt turn; MemoryClient serves files from
// memory and answers permission requests from a script. Abs makes a fixture
// path absolute on the host, as the SDK requires of a path it receives.
package acptest
