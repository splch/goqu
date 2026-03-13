package qasmparse

// tokenType classifies pulse QASM tokens.
type tokenType int

const (
	tokEOF     tokenType = iota
	tokILLEGAL           // unrecognized character / bad token

	// Literals
	tokIDENT    // identifier
	tokNUMBER   // numeric literal (int or float, possibly scientific)
	tokDURATION // numeric literal followed by 's' (e.g., "1e-07s")

	// Delimiters
	tokLPAREN    // (
	tokRPAREN    // )
	tokLBRACKET  // [
	tokRBRACKET  // ]
	tokLBRACE    // {
	tokRBRACE    // }
	tokSEMICOLON // ;
	tokCOMMA     // ,
	tokEQUAL     // =

	// Operators
	tokPLUS  // +
	tokMINUS // -
	tokSTAR  // *
	tokSLASH // /

	// Keywords
	tokOPENQASM
	tokEXTERN
	tokPORT
	tokCAL
	tokFRAME
	tokNEWFRAME
	tokPLAY
	tokDELAY
	tokSET_PHASE
	tokSHIFT_PHASE
	tokSET_FREQUENCY
	tokSHIFT_FREQUENCY
	tokBARRIER
	tokCAPTURE_V0
)

var keywords = map[string]tokenType{
	"OPENQASM":        tokOPENQASM,
	"extern":          tokEXTERN,
	"port":            tokPORT,
	"cal":             tokCAL,
	"frame":           tokFRAME,
	"newframe":        tokNEWFRAME,
	"play":            tokPLAY,
	"delay":           tokDELAY,
	"set_phase":       tokSET_PHASE,
	"shift_phase":     tokSHIFT_PHASE,
	"set_frequency":   tokSET_FREQUENCY,
	"shift_frequency": tokSHIFT_FREQUENCY,
	"barrier":         tokBARRIER,
	"capture_v0":      tokCAPTURE_V0,
}

// token is a lexical token with position information.
type token struct {
	typ     tokenType
	literal string
	line    int
	col     int
}
