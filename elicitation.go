package acp

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"math/big"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The type values of the elicitation property schemas the schema declares.
const (
	propertyTypeString  = "string"
	propertyTypeNumber  = "number"
	propertyTypeInteger = "integer"
	propertyTypeBoolean = "boolean"
	propertyTypeArray   = "array"
)

// The Type of the errors of the elicitation checks.
const (
	errTypeSchema   = "ElicitationSchema"
	errTypeProperty = "ElicitationPropertySchema"
	errTypeAnswer   = "CreateElicitationAccept"
)

// ElicitationField is one property of an elicitation form, as a client shows
// it: its name, whether the form requires it, and its schema
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
type ElicitationField struct {
	Name     string
	Required bool
	Schema   ElicitationPropertySchema
}

// Fields returns the properties of s in the order MarshalJSON writes them:
// each name of PropertyOrder that has a property, once, in that order, then
// the other names sorted. A form decoded from the wire so shows its fields in
// the order the agent wrote them. Required is true for a name s.Required
// lists; a required name with no property gives no field. A property of a type
// this package does not know is a field too, whose Schema.Kind() is its type:
// a client must not render it as a known control
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
func (s ElicitationSchema) Fields() []ElicitationField {
	names := make([]string, 0, len(s.Properties))
	for _, name := range s.PropertyOrder {
		if _, ok := s.Properties[name]; ok && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	fields := make([]ElicitationField, len(names))
	for i, name := range names {
		fields[i] = ElicitationField{Name: name, Required: slices.Contains(s.Required, name), Schema: s.Properties[name]}
	}
	return fields
}

// Kind returns the type of the property schema p holds: "string", "number",
// "integer", "boolean" or "array", the type of one this package does not
// know, or "" when no variant is set
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
func (p ElicitationPropertySchema) Kind() string {
	switch {
	case p.String != nil:
		return propertyTypeString
	case p.Number != nil:
		return propertyTypeNumber
	case p.Integer != nil:
		return propertyTypeInteger
	case p.Boolean != nil:
		return propertyTypeBoolean
	case p.Array != nil:
		return propertyTypeArray
	case p.Other != nil:
		return p.Other.Type
	}
	return ""
}

// Defaults returns the default each property of s declares, typed as an
// answer of that property: a String, Number, Integer, Boolean or StringArray
// value, keyed by the property's name. It is nil when no property declares
// one. A client pre-fills its form with them, as the specification asks; they
// are not an answer until the user sends them. The values are copies
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
func (s ElicitationSchema) Defaults() map[string]ElicitationContentValue {
	var out map[string]ElicitationContentValue
	set := func(name string, v ElicitationContentValue) {
		if out == nil {
			out = make(map[string]ElicitationContentValue)
		}
		out[name] = v
	}
	for name, p := range s.Properties {
		switch {
		case p.String != nil && p.String.Default != nil:
			set(name, NewElicitationContentValueString(*p.String.Default))
		case p.Number != nil && p.Number.Default != nil:
			set(name, NewElicitationContentValueNumber(*p.Number.Default))
		case p.Integer != nil && p.Integer.Default != nil:
			set(name, NewElicitationContentValueInteger(*p.Integer.Default))
		case p.Boolean != nil && p.Boolean.Default != nil:
			set(name, NewElicitationContentValueBoolean(*p.Boolean.Default))
		case p.Array != nil && p.Array.Default != nil:
			set(name, NewElicitationContentValueStringArray(slices.Clone(p.Array.Default)))
		}
	}
	return out
}

// CheckSchema reports what makes s impossible or ambiguous to fill, beyond
// what Validate checks: a required name with no property; a lower bound above
// its upper bound; a list of choices that is empty, repeats a choice, or is
// given as both enum and oneOf; a format other than email, uri, date and
// date-time; a pattern Go's RE2 cannot compile (RuleUnsupported), since JSON
// Schema patterns are ECMA-262; and a default its own property refuses
// (RuleDefault, which also holds the rule it breaks). It returns nil, or each
// problem, a *ValidationError of Type "ElicitationSchema", joined with
// errors.Join in field order. A property of a type this package does not know
// is not checked. The specification does not say who checks a schema, and
// nothing in this package runs it unasked
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 2).
func (s ElicitationSchema) CheckSchema() error {
	var errs []error
	for _, f := range s.Fields() {
		errs = append(errs, checkPropertySchema(f.Name, f.Schema)...)
	}
	for i, name := range s.Required {
		if _, ok := s.Properties[name]; !ok {
			errs = append(errs, schemaError(fmt.Sprintf("required[%d]", i), name, RuleUndeclared, "names no property"))
		}
	}
	return errors.Join(errs...)
}

// schemaError is a problem CheckSchema reports.
func schemaError(field string, value any, rule ValidationRule, text string) *ValidationError {
	return &ValidationError{Type: errTypeSchema, Field: field, Value: value, Rule: rule,
		msg: fmt.Sprintf("%s: %s: %s: %s", errTypeSchema, field, rule, text)}
}

// checkPropertySchema returns the problems of the property schema p of name.
func checkPropertySchema(name string, p ElicitationPropertySchema) []error {
	base := fmt.Sprintf("properties[%q]", name)
	var errs []error
	switch {
	case p.String != nil:
		v := p.String
		if v.MinLength != nil && v.MaxLength != nil && *v.MaxLength < *v.MinLength {
			errs = append(errs, schemaError(base+".maxLength", *v.MaxLength, RuleBounds,
				fmt.Sprintf("maxLength %d is below minLength %d", *v.MaxLength, *v.MinLength)))
		}
		errs = append(errs, checkChoices(base+".enum", v.Enum, false, func(i int) string { return fmt.Sprintf("%s.enum[%d]", base, i) })...)
		consts := optionConsts(v.OneOf)
		errs = append(errs, checkChoices(base+".oneOf", consts, false, func(i int) string { return fmt.Sprintf("%s.oneOf[%d].const", base, i) })...)
		if len(v.Enum) > 0 && len(v.OneOf) > 0 {
			errs = append(errs, schemaError(base, nil, RuleEnum, "has both enum and oneOf"))
		}
		if v.Format != nil && !knownFormat(*v.Format) {
			errs = append(errs, schemaError(base+".format", string(*v.Format), RuleFormat, "is not email, uri, date or date-time"))
		}
		var re *regexp.Regexp
		if v.Pattern != nil {
			var err error
			if re, err = regexp.Compile(*v.Pattern); err != nil {
				errs = append(errs, schemaError(base+".pattern", *v.Pattern, RuleUnsupported, "cannot be compiled by Go's RE2: "+err.Error()))
			}
		}
		if v.Default != nil {
			errs = append(errs, checkDefault(base, p, NewElicitationContentValueString(*v.Default), re)...)
		}
	case p.Number != nil:
		v := p.Number
		if v.Minimum != nil && v.Maximum != nil && *v.Maximum < *v.Minimum {
			errs = append(errs, schemaError(base+".maximum", *v.Maximum, RuleBounds,
				fmt.Sprintf("maximum %s is below minimum %s", formatFloat(*v.Maximum), formatFloat(*v.Minimum))))
		}
		if v.Default != nil {
			errs = append(errs, checkDefault(base, p, NewElicitationContentValueNumber(*v.Default), nil)...)
		}
	case p.Integer != nil:
		v := p.Integer
		if v.Minimum != nil && v.Maximum != nil && *v.Maximum < *v.Minimum {
			errs = append(errs, schemaError(base+".maximum", *v.Maximum, RuleBounds,
				fmt.Sprintf("maximum %d is below minimum %d", *v.Maximum, *v.Minimum)))
		}
		if v.Default != nil {
			errs = append(errs, checkDefault(base, p, NewElicitationContentValueInteger(*v.Default), nil)...)
		}
	case p.Array != nil:
		v := p.Array
		switch {
		case v.Items.String != nil:
			errs = append(errs, checkChoices(base+".items.enum", v.Items.String.Enum, true,
				func(i int) string { return fmt.Sprintf("%s.items.enum[%d]", base, i) })...)
		case v.Items.Titled != nil:
			errs = append(errs, checkChoices(base+".items.anyOf", optionConsts(v.Items.Titled.AnyOf), true,
				func(i int) string { return fmt.Sprintf("%s.items.anyOf[%d].const", base, i) })...)
		}
		if v.MinItems != nil && v.MaxItems != nil && *v.MaxItems < *v.MinItems {
			errs = append(errs, schemaError(base+".maxItems", *v.MaxItems, RuleBounds,
				fmt.Sprintf("maxItems %d is below minItems %d", *v.MaxItems, *v.MinItems)))
		}
		if v.Default != nil {
			errs = append(errs, checkDefault(base, p, NewElicitationContentValueStringArray(v.Default), nil)...)
		}
	}
	return errs
}

// checkChoices reports a list of choices at field that is empty, or that
// repeats a choice, element i being at elem(i). A nil list is not set, which
// is a problem only when the schema requires it.
func checkChoices(field string, values []string, required bool, elem func(i int) string) []error {
	if values == nil && !required {
		return nil
	}
	if len(values) == 0 {
		return []error{schemaError(field, nil, RuleEnum, "is empty")}
	}
	var errs []error
	for i, v := range values {
		if slices.Contains(values[:i], v) {
			errs = append(errs, schemaError(elem(i), v, RuleDuplicate, "repeats an earlier choice"))
		}
	}
	return errs
}

// optionConsts returns the const of each option, nil for nil options.
func optionConsts(opts []EnumOption) []string {
	if opts == nil {
		return nil
	}
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Const
	}
	return out
}

// checkDefault reports the default v of the property p at base when p's own
// rules refuse it, as one RuleDefault error that also holds each rule broken.
// A pattern that does not compile, re nil, is reported on its own.
func checkDefault(base string, p ElicitationPropertySchema, v ElicitationContentValue, re *regexp.Regexp) []error {
	problems := checkValue(p, v, re)
	if len(problems) == 0 {
		return nil
	}
	field := base + ".default"
	texts := make([]string, len(problems))
	inner := make([]error, len(problems))
	for i, pr := range problems {
		texts[i] = fmt.Sprintf("%s: %s", pr.rule, pr.text)
		inner[i] = schemaError(field, nil, pr.rule, pr.text)
	}
	outer := schemaError(field, nil, RuleDefault, "is refused by its own field: "+strings.Join(texts, "; "))
	return []error{&defaultError{outer: outer, inner: inner}}
}

// defaultError is a refused default: the RuleDefault problem first, then each
// rule the default breaks, all reachable with errors.As.
type defaultError struct {
	outer *ValidationError
	inner []error
}

func (e *defaultError) Error() string { return e.outer.Error() }

func (e *defaultError) Unwrap() []error { return append([]error{e.outer}, e.inner...) }

// Check reports whether content, an accepted answer, answers the form s, by
// every rule s states. Each field s requires is present: an empty string or
// list counts as present, as in JSON Schema. Each value is of its field's
// type: a number takes a Number or an Integer, and an integer an Integer or a
// whole Number int64 holds, such as 5.0. Then the field's rules: lengths in
// characters, not bytes; enum and oneOf; the pattern, matched by Go's RE2, one
// RE2 cannot compile reported as RuleUnsupported, never taken to match; the
// format, an email address as RFC 5321 defines it, an absolute URI, an RFC
// 3339 date or date-time; bounds, compared exactly; and item counts and
// choices. A key s does not declare is RuleUndeclared. A value of a type this
// package does not know is not checked. Nothing is trimmed: the value checked
// is the value sent.
//
// It returns nil, or each problem, in field order and then the undeclared
// keys sorted, joined with errors.Join: a *ValidationError of Type
// "CreateElicitationAccept" whose Field is content["<name>"]. Its message
// names the field and the rule, never the value, which is in Value. A client
// checks an answer before it sends accept, and gives the problems back to the
// user; an agent checks the answer it receives, as the specification asks both
// to (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 3).
func (s ElicitationSchema) Check(content map[string]ElicitationContentValue) error {
	var errs []error
	for _, f := range s.Fields() {
		field := fmt.Sprintf("content[%q]", f.Name)
		v, ok := content[f.Name]
		if !ok {
			if f.Required {
				errs = append(errs, answerError(field, nil, RuleRequired, "is required"))
			}
			continue
		}
		var re *regexp.Regexp
		unsupported := false
		if p := f.Schema.String; p != nil && p.Pattern != nil {
			var err error
			re, err = regexp.Compile(*p.Pattern)
			unsupported = err != nil
		}
		problems := checkValue(f.Schema, v, re)
		if unsupported && v.String != nil {
			problems = append(problems, valueProblem{RuleUnsupported, "cannot be checked against its pattern, which Go's RE2 cannot compile"})
		}
		for _, pr := range problems {
			errs = append(errs, answerError(field, v, pr.rule, pr.text))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(content)) {
		if _, ok := s.Properties[name]; !ok {
			errs = append(errs, answerError(fmt.Sprintf("content[%q]", name), content[name], RuleUndeclared, "is not a field of the form"))
		}
	}
	return errors.Join(errs...)
}

// answerError is a problem Check reports.
func answerError(field string, value any, rule ValidationRule, text string) *ValidationError {
	return &ValidationError{Type: errTypeAnswer, Field: field, Value: value, Rule: rule,
		msg: fmt.Sprintf("%s: %s: %s: %s", errTypeAnswer, field, rule, text)}
}

// ParseValue returns the answer value of the property p from text, what a
// text interface collected for it: one string, as it is; a number as
// strconv.ParseFloat reads it, finite; an integer as strconv.ParseInt reads it
// in base 10, so 3.7 and 1e20 are refused, not rounded; a boolean as
// strconv.ParseBool reads it; and a list from every text given. Nothing is
// trimmed. It checks no rule of p but the type; Check does the rest. A
// property of a type this package does not know is refused. A failure is a
// *ValidationError with Rule RuleType, whose message holds no text
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 4).
func (p ElicitationPropertySchema) ParseValue(text ...string) (ElicitationContentValue, error) {
	parseError := func(what string) (ElicitationContentValue, error) {
		return ElicitationContentValue{}, &ValidationError{Type: errTypeProperty, Rule: RuleType,
			msg: errTypeProperty + ": " + string(RuleType) + ": " + what}
	}
	if p.Array != nil {
		return NewElicitationContentValueStringArray(append([]string{}, text...)), nil
	}
	if p.String == nil && p.Number == nil && p.Integer == nil && p.Boolean == nil {
		return parseError(fmt.Sprintf("a property of type %q cannot be parsed", p.Kind()))
	}
	if len(text) != 1 {
		return parseError(fmt.Sprintf("a %s takes one text, not %d", p.Kind(), len(text)))
	}
	t := text[0]
	switch {
	case p.String != nil:
		return NewElicitationContentValueString(t), nil
	case p.Number != nil:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return parseError("the text is not a finite number")
		}
		return NewElicitationContentValueNumber(f), nil
	case p.Integer != nil:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return parseError("the text is not a whole number int64 holds")
		}
		return NewElicitationContentValueInteger(n), nil
	default:
		b, err := strconv.ParseBool(t)
		if err != nil {
			return parseError("the text is not a boolean")
		}
		return NewElicitationContentValueBoolean(b), nil
	}
}

// Float64 returns v as a float64 when it is a Number or an Integer, the
// variant a client's encoder chooses for a whole number; an Integer beyond
// 2^53 is rounded to the nearest float64
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 4).
func (v ElicitationContentValue) Float64() (float64, bool) {
	switch {
	case v.Number != nil:
		return float64(*v.Number), true
	case v.Integer != nil:
		return float64(*v.Integer), true
	}
	return 0, false
}

// Int64 returns v as an int64 when it is an Integer, or a whole Number int64
// holds exactly, such as 5.0, which an encoder may write for an integer field
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 4).
func (v ElicitationContentValue) Int64() (int64, bool) {
	switch {
	case v.Integer != nil:
		return int64(*v.Integer), true
	case v.Number != nil:
		return integral(float64(*v.Number))
	}
	return 0, false
}

// valueProblem is a rule a value breaks, with what a message says of it.
type valueProblem struct {
	rule ValidationRule
	text string
}

// checkValue returns the rules of p that v breaks. A value of the wrong type
// has that problem alone. pattern is p's compiled pattern, nil when p has none
// or it does not compile, which the caller reports. A property of a type this
// package does not know takes any value.
func checkValue(p ElicitationPropertySchema, v ElicitationContentValue, pattern *regexp.Regexp) []valueProblem {
	switch {
	case p.String != nil:
		if v.String == nil {
			return []valueProblem{{RuleType, "is not a string"}}
		}
		return checkString(p.String, string(*v.String), pattern)
	case p.Number != nil:
		return checkNumber(p.Number, v)
	case p.Integer != nil:
		return checkInteger(p.Integer, v)
	case p.Boolean != nil:
		if v.Boolean == nil {
			return []valueProblem{{RuleType, "is not a boolean"}}
		}
	case p.Array != nil:
		if v.StringArray == nil {
			return []valueProblem{{RuleType, "is not a list of strings"}}
		}
		return checkArray(p.Array, *v.StringArray)
	}
	return nil
}

func checkString(p *ElicitationPropertySchemaString, s string, pattern *regexp.Regexp) []valueProblem {
	var out []valueProblem
	n := int64(utf8.RuneCountInString(s))
	if p.MinLength != nil && n < int64(*p.MinLength) {
		out = append(out, valueProblem{RuleMinLength, fmt.Sprintf("is shorter than %d characters", *p.MinLength)})
	}
	if p.MaxLength != nil && n > int64(*p.MaxLength) {
		out = append(out, valueProblem{RuleMaxLength, fmt.Sprintf("is longer than %d characters", *p.MaxLength)})
	}
	if len(p.Enum) > 0 && !slices.Contains(p.Enum, s) {
		out = append(out, valueProblem{RuleEnum, "is not one of its enum values"})
	}
	if len(p.OneOf) > 0 && !slices.Contains(optionConsts(p.OneOf), s) {
		out = append(out, valueProblem{RuleEnum, "is not one of its oneOf options"})
	}
	if pattern != nil && !pattern.MatchString(s) {
		out = append(out, valueProblem{RulePattern, "does not match its pattern"})
	}
	if p.Format != nil && knownFormat(*p.Format) && !matchesFormat(*p.Format, s) {
		out = append(out, valueProblem{RuleFormat, fmt.Sprintf("is not a valid %s", *p.Format)})
	}
	return out
}

func checkNumber(p *ElicitationPropertySchemaNumber, v ElicitationContentValue) []valueProblem {
	var f float64
	var n int64
	isInt := false
	switch {
	case v.Number != nil:
		if f = float64(*v.Number); math.IsNaN(f) || math.IsInf(f, 0) {
			return []valueProblem{{RuleType, "is not a finite number"}}
		}
	case v.Integer != nil:
		n, isInt = int64(*v.Integer), true
	default:
		return []valueProblem{{RuleType, "is not a number"}}
	}
	var out []valueProblem
	if p.Minimum != nil && compareNumber(f, n, isInt, *p.Minimum) < 0 {
		out = append(out, valueProblem{RuleMinimum, "is less than " + formatFloat(*p.Minimum)})
	}
	if p.Maximum != nil && compareNumber(f, n, isInt, *p.Maximum) > 0 {
		out = append(out, valueProblem{RuleMaximum, "is greater than " + formatFloat(*p.Maximum)})
	}
	return out
}

// compareNumber compares a value, n when isInt and f otherwise, with bound,
// exactly: an int64 is compared through math/big, never rounded to float64.
// A NaN bound compares equal, so it bounds nothing.
func compareNumber(f float64, n int64, isInt bool, bound float64) int {
	if math.IsNaN(bound) {
		return 0
	}
	if !isInt {
		switch {
		case f < bound:
			return -1
		case f > bound:
			return 1
		}
		return 0
	}
	return new(big.Float).SetInt64(n).Cmp(big.NewFloat(bound))
}

func checkInteger(p *ElicitationPropertySchemaInteger, v ElicitationContentValue) []valueProblem {
	var n int64
	switch {
	case v.Integer != nil:
		n = int64(*v.Integer)
	case v.Number != nil:
		var ok bool
		if n, ok = integral(float64(*v.Number)); !ok {
			return []valueProblem{{RuleType, "is not an integer"}}
		}
	default:
		return []valueProblem{{RuleType, "is not an integer"}}
	}
	var out []valueProblem
	if p.Minimum != nil && n < *p.Minimum {
		out = append(out, valueProblem{RuleMinimum, fmt.Sprintf("is less than %d", *p.Minimum)})
	}
	if p.Maximum != nil && n > *p.Maximum {
		out = append(out, valueProblem{RuleMaximum, fmt.Sprintf("is greater than %d", *p.Maximum)})
	}
	return out
}

// integral returns f as an int64 when it is a whole number int64 holds
// exactly, as JSON Schema counts 5.0 an integer.
func integral(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || f < -(1<<63) || f >= 1<<63 {
		return 0, false
	}
	return int64(f), true
}

func checkArray(p *ElicitationPropertySchemaArray, items []string) []valueProblem {
	var choices []string
	switch {
	case p.Items.String != nil:
		choices = p.Items.String.Enum
	case p.Items.Titled != nil:
		choices = optionConsts(p.Items.Titled.AnyOf)
	}
	var out []valueProblem
	if choices != nil {
		for i, it := range items {
			if !slices.Contains(choices, it) {
				out = append(out, valueProblem{RuleEnum, fmt.Sprintf("item %d is not one of its options", i)})
			}
		}
	}
	n := uint64(len(items))
	if p.MinItems != nil && n < *p.MinItems {
		out = append(out, valueProblem{RuleMinItems, fmt.Sprintf("has fewer than %d items", *p.MinItems)})
	}
	if p.MaxItems != nil && n > *p.MaxItems {
		out = append(out, valueProblem{RuleMaxItems, fmt.Sprintf("has more than %d items", *p.MaxItems)})
	}
	return out
}

// knownFormat reports whether f is one of the four formats the schema allows.
func knownFormat(f StringFormat) bool {
	switch f {
	case StringFormatEmail, StringFormatUri, StringFormatDate, StringFormatDateTime:
		return true
	}
	return false
}

// matchesFormat reports whether s is of the known format f: an email address,
// an RFC 5321 Mailbox (isMailbox); an absolute URI with no whitespace or
// control character; an RFC 3339 full-date; or an RFC 3339 date-time, whose T
// and Z may be lowercase and whose seconds may be 60, as RFC 3339 allows and
// time.RFC3339 does not.
func matchesFormat(f StringFormat, s string) bool {
	switch f {
	case StringFormatEmail:
		return isMailbox(s)
	case StringFormatUri:
		if strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return false
		}
		u, err := url.Parse(s)
		return err == nil && u.Scheme != ""
	case StringFormatDate:
		_, err := time.Parse(time.DateOnly, s)
		return err == nil
	case StringFormatDateTime:
		_, err := time.Parse(time.RFC3339, rfc3339Strict(s))
		return err == nil
	}
	return true
}

// isMailbox reports whether s is an RFC 5321 §4.1.2 Mailbox, the grammar JSON
// Schema's email format names: a Local-part, a Dot-string of atoms or a
// Quoted-string, then "@", then a Domain of letter-digit-hyphen labels or an
// address literal in brackets, an IPv4 address, "IPv6:" and an IPv6 address,
// or a tag, ":" and its content. Nothing else: no display name, comment or
// space around it, and ASCII only, since non-ASCII addresses are the separate
// idn-email format. Lengths are not checked. It is written here, not with
// net/mail, which would make this package import net
// (0040-PLAN-check-elicitation-forms-answers-and-urls.md, the phase 3
// deviation).
func isMailbox(s string) bool {
	at := strings.LastIndexByte(s, '@')
	if at < 1 || at == len(s)-1 {
		return false
	}
	local, domain := s[:at], s[at+1:]
	return isLocalPart(local) && (isDomain(domain) || isAddressLiteral(domain))
}

// isLetDig reports whether c is an ASCII letter or digit, RFC 5321's Let-dig.
func isLetDig(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

// isAtext reports whether c is an RFC 5321 atext character.
func isAtext(c byte) bool {
	return isLetDig(c) || strings.IndexByte("!#$%&'*+-/=?^_`{|}~", c) >= 0
}

// isLocalPart reports whether s is a Dot-string or a Quoted-string.
func isLocalPart(s string) bool {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		q := s[1 : len(s)-1]
		for i := 0; i < len(q); i++ {
			switch c := q[i]; {
			case c == '\\':
				// quoted-pairSMTP: a backslash and one printable or space.
				i++
				if i == len(q) || q[i] < 32 || q[i] > 126 {
					return false
				}
			case c == '"' || c < 32 || c > 126:
				return false
			}
		}
		return true
	}
	for atom := range strings.SplitSeq(s, ".") {
		if atom == "" {
			return false
		}
		for i := range len(atom) {
			if !isAtext(atom[i]) {
				return false
			}
		}
	}
	return true
}

// isDomain reports whether s is sub-domains joined by dots, each starting and
// ending with a letter or digit and holding only those and hyphens.
func isDomain(s string) bool {
	for label := range strings.SplitSeq(s, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := range len(label) {
			if c := label[i]; c != '-' && !isLetDig(c) {
				return false
			}
		}
	}
	return true
}

// isAddressLiteral reports whether s is an RFC 5321 address literal: an IPv4
// address, "IPv6:" and an IPv6 address, or a standardized tag, ":" and its
// printable content, in brackets.
func isAddressLiteral(s string) bool {
	if len(s) < 3 || s[0] != '[' || s[len(s)-1] != ']' {
		return false
	}
	inner := s[1 : len(s)-1]
	if rest, ok := strings.CutPrefix(inner, "IPv6:"); ok {
		a, err := netip.ParseAddr(rest)
		return err == nil && a.Is6() && a.Zone() == ""
	}
	if a, err := netip.ParseAddr(inner); err == nil {
		return a.Is4()
	}
	tag, content, ok := strings.Cut(inner, ":")
	if !ok || content == "" || !isDomain(tag) {
		return false
	}
	for i := range len(content) {
		if c := content[i]; c < 33 || c > 126 || c == '[' || c == '\\' || c == ']' {
			return false
		}
	}
	return true
}

// rfc3339Strict returns s with a lowercase t separator and z offset in
// uppercase, and a leap second 60 read as 59, so time.RFC3339 parses what RFC
// 3339 §5.6 allows. Anything else is left for the parse to refuse.
func rfc3339Strict(s string) string {
	b := []byte(s)
	if len(b) > 10 && b[10] == 't' {
		b[10] = 'T'
	}
	if n := len(b); n > 0 && b[n-1] == 'z' {
		b[n-1] = 'Z'
	}
	if len(b) > 18 && b[10] == 'T' && b[16] == ':' && b[17] == '6' && b[18] == '0' {
		b[18] = '9'
		b[17] = '5'
	}
	return string(b)
}

// formatFloat writes a bound as the shortest decimal that reads back the same.
func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
