package dictionary

import (
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	const data = "word,phonetic,definition,translation,pos,collins,oxford,tag,bnc,frq,exchange\r\n" +
		"study,stʌdi,,学习\\n研究,,,,,,,s:studies/d:studied\r\n" +
		"studies,,,研究（复数）,,,,,,,0:study\r\n"
	index, err := buildIndex(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input, word, translation string
	}{
		{"“STUDY,”", "study", "学习\n研究"},
		{"studies", "studies", "研究（复数）"},
		{"studied", "study", "学习\n研究"},
		{"study hard", "", ""},
		{"(12)", "", ""},
		{"unknown", "", ""},
	} {
		t.Run(test.input, func(t *testing.T) {
			entry, err := index.lookup(normalizeWord(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if test.word == "" {
				if entry != nil {
					t.Fatalf("expected no match, got %+v", entry)
				}
				return
			}
			if entry == nil || entry.Word != test.word || entry.Translation != test.translation {
				t.Fatalf("unexpected result: %+v", entry)
			}
		})
	}
}

func TestEmbeddedECDICT(t *testing.T) {
	service := &DictionaryService{}
	entry, err := service.Lookup("Study")
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil || entry.Word != "study" || entry.Phonetic == "" || !strings.Contains(entry.Translation, "学习") {
		t.Fatalf("unexpected ECDICT result: %+v", entry)
	}
}
