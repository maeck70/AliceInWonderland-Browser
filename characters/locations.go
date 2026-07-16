package characters

import (
	"regexp"
)

type Location struct {
	Name    string
	Aliases []string
}

type LocationRule struct {
	Name    string
	Regexes []*regexp.Regexp
}

var Locations = []Location{
	{
		Name:    "The Riverbank",
		Aliases: []string{"riverbank", "river-bank", "lying on the bank", "sitting on the bank", "sitting by her sister on the bank"},
	},
	{
		Name:    "The Rabbit Hole",
		Aliases: []string{"rabbit-hole", "rabbit hole"},
	},
	{
		Name:    "The Hall of Doors",
		Aliases: []string{"hall of doors", "locked doors", "long hall", "low hall", "the hall"},
	},
	{
		Name:    "The Pool of Tears",
		Aliases: []string{"pool of tears", "the pool"},
	},
	{
		Name:    "The White Rabbit's House",
		Aliases: []string{"Rabbit's house", "Rabbit's cottage", "house of the Rabbit"},
	},
	{
		Name:    "The Mushroom Path",
		Aliases: []string{"mushroom"},
	},
	{
		Name:    "The Duchess's House",
		Aliases: []string{"Duchess's house", "Duchess's kitchen", "kitchen"},
	},
	{
		Name:    "The March Hare's House",
		Aliases: []string{"March Hare's house", "tea-party", "tea party"},
	},
	{
		Name:    "The Queen's Croquet Ground",
		Aliases: []string{"croquet-ground", "croquet ground", "croquet"},
	},
	{
		Name:    "The Seaside Shore",
		Aliases: []string{"seaside", "shore", "beach"},
	},
	{
		Name:    "The Courtroom",
		Aliases: []string{"court", "courtroom", "trial"},
	},
}

var LocationRules []LocationRule

func InitLocationRules() {
	if len(LocationRules) > 0 {
		return
	}
	for _, loc := range Locations {
		var regexes []*regexp.Regexp
		for _, alias := range loc.Aliases {
			// All locations aliases are matched case-insensitively
			pattern := "(?i)\\b" + regexp.QuoteMeta(alias) + "\\b"
			regexes = append(regexes, regexp.MustCompile(pattern))
		}
		LocationRules = append(LocationRules, LocationRule{
			Name:    loc.Name,
			Regexes: regexes,
		})
	}
}

func DetectLocations(text string) []string {
	var detected []string
	for _, rule := range LocationRules {
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
