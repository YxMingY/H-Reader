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
			word, _ := lookupWords(test.input)
			entry, err := index.lookup(word)
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

func TestLookupLigatures(t *testing.T) {
	service := &DictionaryService{}
	for selection, want := range map[string]string{
		" signiﬁcant": "significant",
		"coﬀee":       "coffee",
		"ﬂower":       "flower",
		"oﬃce":        "office",
		"waﬄe":        "waffle",
		"ﬅudy":        "study",
		"ﬆudy":        "study",
	} {
		t.Run(selection, func(t *testing.T) {
			entry, err := service.Lookup(selection)
			if err != nil {
				t.Fatal(err)
			}
			if entry == nil || entry.Word != want || entry.Translation == "" {
				t.Fatalf("expected %q with translation, got %+v", want, entry)
			}
		})
	}
}

func TestLookupWrappedWords(t *testing.T) {
	service := &DictionaryService{}
	for _, test := range []struct {
		selection, want string
	}{
		{"trans\nlation", "translation"},
		{"TRANS-\nLATION", "translation"},
		{"“trans- \r\n\tlation”", "translation"},
		{"trans\u00ad\nlation", "translation"},
		{"well-\nknown", "well-known"},
		{"well\u2011\r\nknown", "well-known"},
		{"well-known", "well-known"},
		{"trans- lation", ""},
		{"take into\naccount", ""},
		{"trans-\n\nlation", ""},
	} {
		t.Run(test.selection, func(t *testing.T) {
			entry, err := service.Lookup(test.selection)
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if entry != nil {
					t.Fatalf("expected no match, got %+v", entry)
				}
				return
			}
			if entry == nil || entry.Word != test.want || entry.Translation == "" {
				t.Fatalf("expected %q with translation, got %+v", test.want, entry)
			}
		})
	}
}
