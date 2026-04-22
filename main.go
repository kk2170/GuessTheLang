package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Answer int

const (
	Unknown Answer = iota
	Yes
	No
)

type Feature struct {
	Key    string
	Label  string
	Prompt string
}

type Language struct {
	Name     string
	Summary  string
	Features map[string]bool
}

type Match struct {
	Language   Language
	Matches    int
	Mismatches int
	Coverage   int
}

type Game struct {
	candidates []Language
	answers    map[string]Answer
}

var features = []Feature{
	{Key: "static_typing", Label: "静的型付け", Prompt: "静的型付けですか？"},
	{Key: "compiled_native", Label: "ネイティブコンパイル", Prompt: "主にネイティブコードへコンパイルして使いますか？"},
	{Key: "runs_on_jvm", Label: "JVM", Prompt: "JVM 上で動くことが多いですか？"},
	{Key: "runs_on_dotnet", Label: ".NET", Prompt: ".NET ランタイム上で動くことが多いですか？"},
	{Key: "browser_native", Label: "ブラウザ実行", Prompt: "ブラウザでそのまま動くことが多いですか？"},
	{Key: "gc", Label: "GC", Prompt: "ガベージコレクションがありますか？"},
	{Key: "manual_memory", Label: "手動メモリ管理", Prompt: "手動メモリ管理の印象が強いですか？"},
	{Key: "ownership_model", Label: "所有権モデル", Prompt: "所有権や borrow checker が看板機能ですか？"},
	{Key: "indentation_sensitive", Label: "インデント構文", Prompt: "インデントが構文の一部ですか？"},
	{Key: "functional_first", Label: "関数型寄り", Prompt: "関数型の色がかなり強いですか？"},
	{Key: "actor_model", Label: "Actor/軽量プロセス", Prompt: "Actor モデルや軽量プロセスの印象が強いですか？"},
	{Key: "web_backend", Label: "Web バックエンド", Prompt: "Web バックエンドでよく使われますか？"},
	{Key: "mobile_ui", Label: "モバイル開発", Prompt: "モバイルアプリ開発の印象が強いですか？"},
	{Key: "apple_ecosystem", Label: "Apple エコシステム", Prompt: "Apple 系開発で存在感が大きいですか？"},
	{Key: "superset_of_js", Label: "JavaScript 上位互換", Prompt: "JavaScript の上位互換として使われますか？"},
	{Key: "dollar_variables", Label: "$ 変数", Prompt: "$variable のような記法をよく使いますか？"},
	{Key: "null_safety_focus", Label: "null 安全性", Prompt: "null 安全性が強く打ち出されていますか？"},
	{Key: "c_family_syntax", Label: "C 系の波括弧構文", Prompt: "C 系の波括弧構文ですか？"},
	{Key: "oop_impression", Label: "クラスベース OOP", Prompt: "クラスベース OOP の印象が強いですか？"},
	{Key: "lightweight_concurrency", Label: "軽量並行処理", Prompt: "軽量並行処理が看板機能の 1 つですか？"},
}

var languages = []Language{
	language("Go", "シンプルな文法と goroutine による軽量並行処理が特徴です。", "static_typing", "compiled_native", "gc", "web_backend", "c_family_syntax", "lightweight_concurrency"),
	language("Rust", "所有権モデルで安全性と性能を両立しやすい言語です。", "static_typing", "compiled_native", "ownership_model", "c_family_syntax"),
	language("C", "低レベル制御と手動メモリ管理の印象が強い言語です。", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
	language("C++", "C 系の性能志向とクラスベース OOP を併せ持つ言語です。", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	language("Java", "JVM 上で動き、業務システムやバックエンドで定番の言語です。", "static_typing", "runs_on_jvm", "gc", "web_backend", "c_family_syntax", "oop_impression"),
	language("C#", ".NET を中心に使われるクラスベース OOP の代表的な言語です。", "static_typing", "runs_on_dotnet", "gc", "c_family_syntax", "oop_impression"),
	language("JavaScript", "ブラウザでそのまま動き、Web 開発の中心にいる言語です。", "browser_native", "gc", "web_backend", "c_family_syntax"),
	language("TypeScript", "JavaScript の上位互換として使われる静的型付き言語です。", "static_typing", "browser_native", "gc", "web_backend", "superset_of_js", "c_family_syntax"),
	language("Python", "読みやすさとインデント構文で知られる汎用言語です。", "gc", "web_backend", "indentation_sensitive", "oop_impression"),
	language("Ruby", "書き味の良さと Web 開発の印象が強い動的言語です。", "gc", "web_backend", "oop_impression"),
	language("PHP", "サーバーサイド Web との結びつきが強いスクリプト言語です。", "gc", "web_backend", "dollar_variables"),
	language("Swift", "Apple 系開発で存在感が大きいネイティブ志向の言語です。", "static_typing", "compiled_native", "apple_ecosystem", "null_safety_focus", "c_family_syntax", "oop_impression"),
	language("Kotlin", "JVM と Android の文脈で人気が高く、null 安全性も強い言語です。", "static_typing", "runs_on_jvm", "gc", "mobile_ui", "null_safety_focus", "c_family_syntax", "oop_impression"),
	language("Haskell", "純粋関数型の色が強く、型システムでも知られる言語です。", "static_typing", "compiled_native", "gc", "functional_first"),
	language("Scala", "JVM 上で動く関数型寄りのマルチパラダイム言語です。", "static_typing", "runs_on_jvm", "gc", "functional_first", "oop_impression"),
	language("Elixir", "BEAM 上で動き、Actor モデルや軽量プロセスの印象が強い言語です。", "gc", "functional_first", "actor_model", "web_backend", "lightweight_concurrency"),
	language("Dart", "Flutter 文脈でよく見かける、モバイル寄りの言語です。", "static_typing", "gc", "mobile_ui", "null_safety_focus", "c_family_syntax", "oop_impression"),
}

func main() {
	game := Game{
		candidates: append([]Language(nil), languages...),
		answers:    make(map[string]Answer),
	}

	if err := run(game, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(game Game, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "Guess The Lang")
	fmt.Fprintln(out, "Yes / No で答えてください。分からない場合は ? でスキップできます。")
	fmt.Fprintln(out)

	for {
		remaining := remainingCandidates(game.candidates, game.answers)

		switch len(remaining) {
		case 0:
			reportNoExactMatch(out, game.answers)
			return nil
		case 1:
			reportSingleGuess(out, remaining[0], game.answers)
			return nil
		}

		feature, ok := bestQuestion(remaining, game.answers)
		if !ok {
			reportAmbiguousResult(out, remaining)
			return nil
		}

		answer, err := ask(reader, out, feature)
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(out)
				return nil
			}
			return err
		}

		game.answers[feature.Key] = answer
	}
}

func language(name, summary string, featureKeys ...string) Language {
	features := make(map[string]bool, len(featureKeys))
	for _, key := range featureKeys {
		features[key] = true
	}
	return Language{Name: name, Summary: summary, Features: features}
}

func ask(reader *bufio.Reader, out io.Writer, feature Feature) (Answer, error) {
	for {
		fmt.Fprintf(out, "- %s [y/n/?]: ", feature.Prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					return Unknown, io.EOF
				}
				answer, ok := parseAnswer(trimmed)
				if ok {
					return answer, nil
				}
				return Unknown, fmt.Errorf("invalid answer: %q", trimmed)
			}
			return Unknown, err
		}

		answer, ok := parseAnswer(line)
		if ok {
			return answer, nil
		}

		fmt.Fprintln(out, "  y / n / ? のどれかで答えてください。")
	}
}

func parseAnswer(raw string) (Answer, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "y", "yes", "はい", "1":
		return Yes, true
	case "n", "no", "いいえ", "0":
		return No, true
	case "?", "s", "skip", "わからない", "unknown":
		return Unknown, true
	default:
		return Unknown, false
	}
}

func remainingCandidates(candidates []Language, answers map[string]Answer) []Language {
	filtered := make([]Language, 0, len(candidates))
	for _, candidate := range candidates {
		if matchesAnswers(candidate, answers) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func matchesAnswers(language Language, answers map[string]Answer) bool {
	for key, answer := range answers {
		if answer == Unknown {
			continue
		}
		hasFeature := language.Features[key]
		if answer == Yes && !hasFeature {
			return false
		}
		if answer == No && hasFeature {
			return false
		}
	}
	return true
}

func bestQuestion(candidates []Language, answers map[string]Answer) (Feature, bool) {
	bestScore := -1
	var selected Feature

	for _, feature := range features {
		if _, alreadyAsked := answers[feature.Key]; alreadyAsked {
			continue
		}

		yesCount := 0
		noCount := 0
		for _, candidate := range candidates {
			if candidate.Features[feature.Key] {
				yesCount++
			} else {
				noCount++
			}
		}

		if yesCount == 0 || noCount == 0 {
			continue
		}

		score := yesCount * noCount
		if score > bestScore {
			bestScore = score
			selected = feature
		}
	}

	return selected, bestScore >= 0
}

func reportSingleGuess(out io.Writer, candidate Language, answers map[string]Answer) {
	fmt.Fprintln(out)
	fmt.Fprintf(out, "たぶん %s です。\n", candidate.Name)
	if highlights := matchingHighlights(candidate, answers, 4); len(highlights) > 0 {
		fmt.Fprintf(out, "根拠: %s\n", strings.Join(highlights, " / "))
	}
	fmt.Fprintf(out, "ひとこと: %s\n", candidate.Summary)
}

func reportAmbiguousResult(out io.Writer, candidates []Language) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "かなり絞れましたが、今の質問セットだけでは一意に決め切れませんでした。")
	fmt.Fprintln(out, "候補:")
	for _, candidate := range candidates {
		fmt.Fprintf(out, "- %s: %s\n", candidate.Name, candidate.Summary)
	}
	fmt.Fprintln(out, "質問項目を増やすと、さらに精度を上げられます。")
}

func reportNoExactMatch(out io.Writer, answers map[string]Answer) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "手元のデータでは完全一致が見つかりませんでした。近い候補は次のあたりです。")
	for _, match := range rankMatches(languages, answers, 3) {
		fmt.Fprintf(out, "- %s (%d 一致 / %d 不一致): %s\n", match.Language.Name, match.Matches, match.Mismatches, match.Language.Summary)
	}
}

func rankMatches(candidates []Language, answers map[string]Answer, limit int) []Match {
	matches := make([]Match, 0, len(candidates))
	for _, candidate := range candidates {
		match := Match{Language: candidate}
		for key, answer := range answers {
			if answer == Unknown {
				continue
			}
			match.Coverage++
			hasFeature := candidate.Features[key]
			if (answer == Yes && hasFeature) || (answer == No && !hasFeature) {
				match.Matches++
			} else {
				match.Mismatches++
			}
		}
		matches = append(matches, match)
	}

	sort.Slice(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		if left.Mismatches != right.Mismatches {
			return left.Mismatches < right.Mismatches
		}
		if left.Matches != right.Matches {
			return left.Matches > right.Matches
		}
		return left.Language.Name < right.Language.Name
	})

	if limit > len(matches) {
		limit = len(matches)
	}
	return matches[:limit]
}

func matchingHighlights(candidate Language, answers map[string]Answer, limit int) []string {
	labels := make([]string, 0, limit)
	for _, feature := range features {
		if len(labels) >= limit {
			break
		}
		if answers[feature.Key] == Yes && candidate.Features[feature.Key] {
			labels = append(labels, feature.Label)
		}
	}
	return labels
}
