// Package lint provides spell-checking services for plain text.
// Document depends only on the Linter interface, so implementations
// are interchangeable (Strategy pattern on the class diagram).
package lint

// Options configures a single check pass.
type Options struct {
	// Language selects the dictionary, e.g. "ru". Empty means the default.
	Language string
	// MaxSuggestions limits corrections offered per error. 0 means default.
	MaxSuggestions int
}

// Error describes one misspelled word.
// Start and End are byte offsets into the checked plain text, half-open.
type Error struct {
	Start, End  int
	Word        string
	Message     string
	Suggestions []string
}

// Result is the outcome of one Check call.
type Result struct {
	Errors       []Error
	Success      bool
	ErrorMessage string
}

// Linter checks plain text for spelling errors. The contract is
// "given text — return errors"; it never modifies the text itself.
type Linter interface {
	Check(plainText string, opts Options) Result
}

// Suggester is an optional capability of a Linter: computing corrections
// for one misspelled word. Suggestions are expensive, so they are produced
// lazily — only when the user asks for them — rather than during Check.
type Suggester interface {
	// Suggest returns up to max corrections for word. max <= 0 means
	// the implementation's default.
	Suggest(word string, max int) []string
}
