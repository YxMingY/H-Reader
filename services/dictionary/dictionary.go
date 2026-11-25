package dictionary

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

//go:embed ecdict.csv
var ecdictCSV string

const (
	wordColumn        = 0
	phoneticColumn    = 1
	translationColumn = 3
	exchangeColumn    = 10
)

// Entry is a local English–Chinese dictionary result.
type Entry struct {
	Word        string `json:"word"`
	Phonetic    string `json:"phonetic"`
	Translation string `json:"translation"`
}

type dictionaryIndex struct {
	data        string
	words       map[string]uint32
	inflections map[string]uint32
}

var (
	englishWord = regexp.MustCompile(`^[a-z]+(?:['-][a-z]+)*$`)
	lineBreak   = regexp.MustCompile(`[\t\p{Zs}]*(?:\r\n|[\r\n\x{2028}])[\t\p{Zs}]*`)
	punctuation = strings.NewReplacer("’", "'", "‘", "'", "‑", "-", "‐", "-")
	loadIndex   = sync.OnceValues(func() (*dictionaryIndex, error) { return buildIndex(ecdictCSV) })
)

type DictionaryService struct{}

// Prepare warms the dictionary index without blocking the reader UI.
func (*DictionaryService) Prepare() error {
	_, err := loadIndex()
	return err
}

// Lookup returns the exact entry or a matching base word from ECDICT.
func (*DictionaryService) Lookup(selection string) (*Entry, error) {
	word, unhyphenated := lookupWords(selection)
	if word == "" {
		return nil, nil
	}
	index, err := loadIndex()
	if err != nil {
		return nil, err
	}
	entry, err := index.lookup(word)
	if err != nil || entry != nil || unhyphenated == "" {
		return entry, err
	}
	return index.lookup(unhyphenated)
}

// lookupWords returns a preferred spelling and, for line-end hyphens, a fallback.
func lookupWords(selection string) (string, string) {
	word := strings.TrimFunc(selection, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	word = strings.ToLower(punctuation.Replace(word))
	if englishWord.MatchString(word) {
		return word, ""
	}

	parts := lineBreak.Split(word, -1)
	if len(parts) < 2 {
		return "", ""
	}
	for i := 0; i < len(parts)-1; i++ {
		// A soft hyphen already marks a discretionary line break.
		parts[i] = strings.TrimSuffix(parts[i], "\u00ad")
		if parts[i] == "" {
			return "", ""
		}
	}
	if parts[len(parts)-1] == "" {
		return "", ""
	}
	word = strings.Join(parts, "")
	if !englishWord.MatchString(word) {
		return "", ""
	}

	for i := 0; i < len(parts)-1; i++ {
		parts[i] = strings.TrimSuffix(parts[i], "-")
	}
	alternative := strings.Join(parts, "")
	if alternative == word || !englishWord.MatchString(alternative) {
		return word, ""
	}
	return word, alternative
}

func buildIndex(data string) (*dictionaryIndex, error) {
	reader := csv.NewReader(strings.NewReader(data))
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read ECDICT header: %w", err)
	}
	if len(header) <= exchangeColumn || header[wordColumn] != "word" ||
		header[phoneticColumn] != "phonetic" || header[translationColumn] != "translation" ||
		header[exchangeColumn] != "exchange" {
		return nil, fmt.Errorf("unexpected ECDICT CSV header")
	}

	index := &dictionaryIndex{
		data:        data,
		words:       make(map[string]uint32),
		inflections: make(map[string]uint32),
	}
	for {
		offset := uint32(reader.InputOffset())
		record, err := reader.Read()
		if err == io.EOF {
			return index, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read ECDICT at offset %d: %w", offset, err)
		}

		word := strings.ToLower(record[wordColumn])
		if !englishWord.MatchString(word) || strings.TrimSpace(record[translationColumn]) == "" {
			continue
		}
		if _, exists := index.words[word]; !exists {
			// Clone keys so the index does not retain each CSV row's other fields.
			index.words[strings.Clone(word)] = offset
		}
		for exchange := range strings.SplitSeq(record[exchangeColumn], "/") {
			kind, form, ok := strings.Cut(exchange, ":")
			if !ok || kind == "0" || kind == "1" {
				continue
			}
			form = strings.ToLower(form)
			if _, exists := index.inflections[form]; !exists && englishWord.MatchString(form) {
				index.inflections[strings.Clone(form)] = offset
			}
		}
	}
}

func (index *dictionaryIndex) lookup(word string) (*Entry, error) {
	offset, ok := index.words[word]
	if !ok {
		offset, ok = index.inflections[word]
	}
	if !ok {
		return nil, nil
	}
	record, err := csv.NewReader(strings.NewReader(index.data[offset:])).Read()
	if err != nil {
		return nil, fmt.Errorf("read ECDICT entry for %q: %w", word, err)
	}
	return &Entry{
		Word:        record[wordColumn],
		Phonetic:    record[phoneticColumn],
		Translation: strings.ReplaceAll(record[translationColumn], `\n`, "\n"),
	}, nil
}
