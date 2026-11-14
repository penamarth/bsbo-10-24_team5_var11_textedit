package lint

type Options struct {
	Language string

	MaxSuggestions int
}

type Error struct {
	Start, End  int
	Word        string
	Message     string
	Suggestions []string
}

type Result struct {
	Errors       []Error
	Success      bool
	ErrorMessage string
}

type Linter interface {
	Check(plainText string, opts Options) Result
}

type Suggester interface {
	Suggest(word string, max int) []string
}
