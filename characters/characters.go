package characters

import (
	"regexp"
	"strings"
)

type Character struct {
	Name    string
	Aliases []string
}

type CharacterRule struct {
	Name    string
	Regexes []*regexp.Regexp
}

var Characters = []Character{
	{Name: "Alice", Aliases: []string{"Alice"}},
	{Name: "White Rabbit", Aliases: []string{"White Rabbit", "Rabbit"}},
	{Name: "Mouse", Aliases: []string{"Mouse"}},
	{Name: "Dodo", Aliases: []string{"Dodo"}},
	{Name: "Lory", Aliases: []string{"Lory"}},
	{Name: "Eaglet", Aliases: []string{"Eaglet"}},
	{Name: "Duck", Aliases: []string{"Duck"}},
	{Name: "Pat", Aliases: []string{"Pat"}},
	{Name: "Bill", Aliases: []string{"Bill"}},
	{Name: "Puppy", Aliases: []string{"Puppy"}},
	{Name: "Caterpillar", Aliases: []string{"Caterpillar"}},
	{Name: "Duchess", Aliases: []string{"Duchess"}},
	{Name: "Cheshire Cat", Aliases: []string{"Cheshire Cat", "Cheshire", "Cat"}},
	{Name: "March Hare", Aliases: []string{"March Hare", "Hare"}},
	{Name: "Hatter", Aliases: []string{"Hatter", "Mad Hatter"}},
	{Name: "Dormouse", Aliases: []string{"Dormouse"}},
	{Name: "Queen of Hearts", Aliases: []string{"Queen of Hearts", "Queen"}},
	{Name: "King of Hearts", Aliases: []string{"King of Hearts", "King"}},
	{Name: "Knave of Hearts", Aliases: []string{"Knave of Hearts", "Knave"}},
	{Name: "Gryphon", Aliases: []string{"Gryphon"}},
	{Name: "Mock Turtle", Aliases: []string{"Mock Turtle", "Turtle"}},
	{Name: "Dinah", Aliases: []string{"Dinah"}},
	{Name: "Elsie", Aliases: []string{"Elsie"}},
	{Name: "Lacie", Aliases: []string{"Lacie"}},
	{Name: "Tillie", Aliases: []string{"Tillie"}},
	{Name: "Mary Ann", Aliases: []string{"Mary Ann"}},
	{Name: "Cook", Aliases: []string{"Cook"}},
	{Name: "Frog-Footman", Aliases: []string{"Frog-Footman", "Frog"}},
	{Name: "Fish-Footman", Aliases: []string{"Fish-Footman", "Fish"}},
	{Name: "Pigeon", Aliases: []string{"Pigeon"}},
	{Name: "Alice's Sister", Aliases: []string{"sister", "Sister"}},
}

var Rules []CharacterRule

func InitRules() {
	if len(Rules) > 0 {
		return
	}
	for _, char := range Characters {
		var regexes []*regexp.Regexp
		for _, alias := range char.Aliases {
			var pattern string
			if strings.ToLower(alias) == "sister" {
				pattern = "(?i)\\b" + regexp.QuoteMeta(alias) + "\\b"
			} else {
				pattern = "\\b" + regexp.QuoteMeta(alias) + "\\b"
			}
			regexes = append(regexes, regexp.MustCompile(pattern))
		}
		Rules = append(Rules, CharacterRule{
			Name:    char.Name,
			Regexes: regexes,
		})
	}
}

func DetectCharacters(text string) []string {
	var detected []string
	for _, rule := range Rules {
		matched := false
		for _, re := range rule.Regexes {
			if re.MatchString(text) {
				matched = true
				break
			}
		}
		if matched {
			detected = append(detected, rule.Name)
		}
	}
	return detected
}
