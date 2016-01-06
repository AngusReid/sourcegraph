// Package filelang detects the programming language of files by
// their filenames and (probabilistically/heuristically) their
// contents.
package filelang

import (
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

//go:generate go run generate.go

// A Language represents a programming or markup language.
//
// See
// https://github.com/github/filelang/blob/master/lib/filelang/languages.yml
// for a description of each field.
type Language struct {
	Name string `yaml:"-"`

	Type         string   `yaml:"type,omitempty"`
	Aliases      []string `yaml:"aliases,omitempty"`
	Wrap         bool     `yaml:"wrap,omitempty"`
	Extensions   []string `yaml:"extensions,omitempty"`
	Interpreters []string `yaml:"interpreters,omitempty"`
	Searchable   bool     `yaml:"searchable,omitempty"`
	Color        string   `yaml:"color,omitempty"`
	Group        string   `yaml:"group,omitempty"`
	Filenames    []string `yaml:"filenames,omitempty"`

	// Omitted fields:
	//  - ace_mode
	//  - search_term
	//  - tm_scope
}

// MatchFilename returns true if the language is associated with files
// with the given name.
func (l *Language) MatchFilename(name string) bool {
	for _, n := range l.Filenames {
		if name == n {
			return true
		}
	}
	if ext := path.Ext(name); ext != "" {
		for _, x := range l.Extensions {
			if strings.EqualFold(ext, x) {
				return true
			}
		}
	}
	return false
}

// primaryMatchFilename returns true if name's extension equals l's
// primary extension or if name matches one of l's filenames.
func (l *Language) primaryMatchFilename(name string) bool {
	for _, n := range l.Filenames {
		if name == n {
			return true
		}
	}
	if ext := path.Ext(name); ext != "" && len(l.Extensions) > 0 {
		return strings.EqualFold(l.Extensions[0], ext)
	}
	return false
}

// Languages is a dataset of languages.
type Languages []*Language

func (ls Languages) MarshalYAML() (interface{}, error) {
	m := make(yaml.MapSlice, len(ls))
	for i, l := range ls {
		m[i] = yaml.MapItem{
			Key:   l.Name,
			Value: l,
		}
	}
	return m, nil
}

func (ls *Languages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Note: does not preserve order.

	// var m map[string]Language
	// if err := unmarshal(&m); err != nil {
	// 	return err
	// }
	// *ls = make(Languages, 0, len(m))
	// for name, lang := range m {
	// 	lang.Name = name
	// 	*ls = append(*ls, lang)
	// }
	// return nil

	// Preserves order.

	var m yaml.MapSlice
	if err := unmarshal(&m); err != nil {
		return err
	}
	*ls = make(Languages, len(m))
	for i, mi := range m {
		b, err := yaml.Marshal(mi.Value)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(b, &(*ls)[i]); err != nil {
			return err
		}
		(*ls)[i].Name = mi.Key.(string)
	}
	return nil
}

// ByFilename returns a list of languages associated with the given
// filename or its extension.
func (ls Languages) ByFilename(name string) []*Language {
	var matches []*Language
	for _, l := range ls {
		if l.MatchFilename(name) {
			matches = append(matches, l)
		}
	}
	sort.Sort(&sortByPrimaryMatch{name, matches})
	return matches
}

// sortByPrimaryMatch sorts matches first by whether the filename is a
// primary match (of the language's filenames or the language's
// primary extension) and then alphabetically by language name.
type sortByPrimaryMatch struct {
	name  string
	langs []*Language
}

func (v *sortByPrimaryMatch) Len() int { return len(v.langs) }

func (v *sortByPrimaryMatch) Less(i, j int) bool {
	ipm := v.langs[i].primaryMatchFilename(v.name)
	jpm := v.langs[j].primaryMatchFilename(v.name)
	if ipm == jpm {
		return v.langs[i].Name < v.langs[j].Name
	}
	return ipm
}

func (v *sortByPrimaryMatch) Swap(i, j int) { v.langs[i], v.langs[j] = v.langs[j], v.langs[i] }
