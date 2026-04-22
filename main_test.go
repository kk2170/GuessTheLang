package main

import (
	"bytes"
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

	got := remainingCandidates(languages, answers)
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

func TestRunRetriesInvalidInputAndFindsCPP(t *testing.T) {
	game := Game{
		candidates: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers: map[string]Answer{},
	}

	var out bytes.Buffer
	err := run(game, strings.NewReader("maybe\ny\n"), &out)
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
		candidates: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers: map[string]Answer{},
	}

	var out bytes.Buffer
	err := run(game, strings.NewReader("?\n"), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "一意に決め切れませんでした") {
		t.Fatalf("expected ambiguous result message, got %q", out.String())
	}
}

func TestRunReportsNoExactMatch(t *testing.T) {
	game := Game{
		candidates: []Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		answers: map[string]Answer{
			"manual_memory": No,
		},
	}

	var out bytes.Buffer
	err := run(game, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "完全一致が見つかりませんでした") {
		t.Fatalf("expected no exact match message, got %q", out.String())
	}
}
