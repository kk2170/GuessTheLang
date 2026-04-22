package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAnswer(t *testing.T) {
	tests := []struct {
		input string
		want  Answer
		ok    bool
	}{
		{input: "y", want: Yes, ok: true},
		{input: "はい", want: Yes, ok: true},
		{input: "no", want: No, ok: true},
		{input: "?", want: Unknown, ok: true},
		{input: "maybe", want: Unknown, ok: false},
	}

	for _, tc := range tests {
		got, ok := parseAnswer(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("parseAnswer(%q) = (%v, %v), want (%v, %v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRemainingCandidatesIdentifiesGo(t *testing.T) {
	answers := map[string]Answer{
		"static_typing":           Yes,
		"compiled_native":         Yes,
		"gc":                      Yes,
		"web_backend":             Yes,
		"lightweight_concurrency": Yes,
	}

	got := remainingCandidates(builtinLanguages, answers)
	if len(got) != 1 {
		var names []string
		for _, candidate := range got {
			names = append(names, candidate.Name)
		}
		t.Fatalf("expected exactly one match, got %v", names)
	}

	if got[0].Name != "Go" {
		t.Fatalf("expected Go, got %s", got[0].Name)
	}
}

func TestBestQuestionForCandCppPair(t *testing.T) {
	candidates := []Language{
		language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
		language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	}

	answers := map[string]Answer{
		"static_typing":   Yes,
		"compiled_native": Yes,
		"manual_memory":   Yes,
		"c_family_syntax": Yes,
	}

	question, ok := bestQuestion(candidates, answers)
	if !ok {
		t.Fatal("expected a discriminating question")
	}

	if question.Key != "oop_impression" {
		t.Fatalf("expected oop_impression, got %s", question.Key)
	}
}

func TestSaveAndLoadLearnedLanguages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.json")
	learned := []Language{
		language("Zig", "A systems language.", "static_typing", "compiled_native"),
	}

	if err := saveLearnedLanguages(path, learned); err != nil {
		t.Fatalf("saveLearnedLanguages returned error: %v", err)
	}

	loaded, err := loadLearnedLanguages(path)
	if err != nil {
		t.Fatalf("loadLearnedLanguages returned error: %v", err)
	}

	if len(loaded) != 1 || loaded[0].Name != "Zig" {
		t.Fatalf("unexpected learned languages: %+v", loaded)
	}
	if !loaded[0].Features["static_typing"] || !loaded[0].Features["compiled_native"] {
		t.Fatalf("expected persisted features to round-trip, got %+v", loaded[0].Features)
	}
}

func TestRunRetriesInvalidInputAndFindsCPP(t *testing.T) {
	game := Game{
		languages: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers:       map[string]Answer{},
		knowledgePath: filepath.Join(t.TempDir(), "knowledge.json"),
	}

	var out bytes.Buffer
	err := run(&game, strings.NewReader("maybe\ny\n"), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "y / n / ? のどれか") {
		t.Fatalf("expected invalid input retry message, got %q", got)
	}
	if !strings.Contains(got, "たぶん C++ です。") {
		t.Fatalf("expected C++ guess, got %q", got)
	}
}

func TestRunReportsAmbiguousWhenQuestionSkipped(t *testing.T) {
	game := Game{
		languages: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers:       map[string]Answer{},
		knowledgePath: filepath.Join(t.TempDir(), "knowledge.json"),
	}

	var out bytes.Buffer
	err := run(&game, strings.NewReader("?\n"), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "一意に決め切れませんでした") {
		t.Fatalf("expected ambiguous result message, got %q", out.String())
	}
}

func TestRunReportsNoExactMatch(t *testing.T) {
	game := Game{
		languages: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers: map[string]Answer{
			"manual_memory": No,
		},
		knowledgePath: filepath.Join(t.TempDir(), "knowledge.json"),
	}

	var out bytes.Buffer
	err := run(&game, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "完全一致が見つかりませんでした") {
		t.Fatalf("expected no exact match message, got %q", out.String())
	}
}

func TestRunLearnsLanguageOnWrongGuess(t *testing.T) {
	knowledgePath := filepath.Join(t.TempDir(), "knowledge.json")
	game := Game{
		languages: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers:       map[string]Answer{},
		knowledgePath: knowledgePath,
	}

	var out bytes.Buffer
	input := "n\nn\nZig\nA safer systems language.\n" + strings.Repeat("?\n", len(features)-1)
	err := run(&game, strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "学習しました: Zig") {
		t.Fatalf("expected learning confirmation, got %q", out.String())
	}

	loaded, err := loadLearnedLanguages(knowledgePath)
	if err != nil {
		t.Fatalf("loadLearnedLanguages returned error: %v", err)
	}

	if len(loaded) != 1 || loaded[0].Name != "Zig" {
		t.Fatalf("expected Zig to be learned, got %+v", loaded)
	}
}

func TestRunDoesNotOverrideBuiltinLanguage(t *testing.T) {
	knowledgePath := filepath.Join(t.TempDir(), "knowledge.json")
	game := Game{
		languages: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers:       map[string]Answer{},
		knowledgePath: knowledgePath,
	}

	var out bytes.Buffer
	input := "n\nn\nGo\n"
	err := run(&game, strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "初期収録済み") {
		t.Fatalf("expected builtin-language warning, got %q", out.String())
	}

	loaded, err := loadLearnedLanguages(knowledgePath)
	if err != nil {
		t.Fatalf("loadLearnedLanguages returned error: %v", err)
	}

	if len(loaded) != 0 {
		t.Fatalf("expected no learned languages, got %+v", loaded)
	}
}
