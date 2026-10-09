// Package emit writes Go sources from the generator's intermediate schema.
package emit

import jen "github.com/dave/jennifer/jen"

const (
	schemaTypeObject  = "object"
	schemaTypeArray   = "array"
	schemaTypeString  = "string"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
	schemaTypeBoolean = "boolean"
	schemaTypeNull    = "null"
	jsonStructTag     = "json"
	unionContentBlock = "ContentBlock"
	wireSessionCancel = "session/cancel"
	nameSessionUpdate = "SessionUpdate"
	generatedFileMode = 0o600
)

// Code is an alias for a jennifer code fragment.
type Code = jen.Code

// Dict is an alias for a jennifer dictionary.
type Dict = jen.Dict

// Group is an alias for a jennifer statement group.
type Group = jen.Group

// File is an alias for a jennifer file.
type File = jen.File

var (
	// NewFile starts a jennifer file for the named package.
	NewFile = jen.NewFile
	// ID constructs a jennifer identifier.
	ID = jen.Id
	// Lit constructs a jennifer literal.
	Lit = jen.Lit
	// Line emits a blank line in jennifer output.
	Line = jen.Line
	// Func constructs a jennifer function declaration.
	Func = jen.Func
	// For constructs a jennifer for statement.
	For = jen.For
	// Range constructs a jennifer range statement.
	Range = jen.Range
	// Return constructs a jennifer return statement.
	Return = jen.Return
	// Nil is jennifer's nil identifier.
	Nil = jen.Nil
	// String is jennifer's string type.
	String = jen.String
	// Int is jennifer's int type.
	Int = jen.Int
	// Float64 is jennifer's float64 type.
	Float64 = jen.Float64
	// Bool is jennifer's bool type.
	Bool = jen.Bool
	// Any is jennifer's any type.
	Any = jen.Any
	// Map constructs a jennifer map type.
	Map = jen.Map
	// Index constructs a jennifer slice type.
	Index = jen.Index
	// Qual constructs a jennifer qualified identifier.
	Qual = jen.Qual
	// Error is jennifer's error type.
	Error = jen.Error
	// Case constructs a jennifer case clause.
	Case = jen.Case
	// Default constructs a jennifer default clause.
	Default = jen.Default
	// Switch constructs a jennifer switch statement.
	Switch = jen.Switch
	// Var constructs a jennifer variable declaration.
	Var = jen.Var
	// If constructs a jennifer if statement.
	If = jen.If
	// List constructs a jennifer expression list.
	List = jen.List
	// Op constructs a jennifer operator.
	Op = jen.Op
	// InterfaceFunc builds a jennifer interface with a callback.
	InterfaceFunc = jen.InterfaceFunc
	// Comment constructs a jennifer comment.
	Comment = jen.Comment
)
