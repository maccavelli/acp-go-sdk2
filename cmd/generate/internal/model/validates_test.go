package model

// Validates reports whether the named Go type has a Validate method. Only the
// model's tests ask, so it lives beside them; the generator reads the model's
// Validation and Validates fields instead
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G13).
func (m *Model) Validates(name string) bool { return m.validates[name] }
