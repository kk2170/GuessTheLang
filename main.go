package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type persistedLanguage struct {
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Features []string `json:"features"`
}

type knowledgeFile struct {
	Languages []persistedLanguage `json:"languages"`
}

type Match struct {
	Language   Language
	Matches    int
	Mismatches int
	Coverage   int
}

type Game struct {
	languages     []Language
	learned       []Language
	answers       map[string]Answer
	knowledgePath string
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
	{Key: "shell_scripting", Label: "シェルスクリプト", Prompt: "シェルスクリプトとして使う印象が強いですか？"},
	{Key: "lisp_syntax", Label: "Lisp 系構文", Prompt: "Lisp 系の括弧中心構文ですか？"},
	{Key: "scientific_computing", Label: "科学技術計算", Prompt: "科学技術計算やデータ分析の印象が強いですか？"},
}

var builtinLanguages = []Language{
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
	language("Bash", "コマンドライン自動化でよく使われるシェルスクリプト言語です。", "shell_scripting"),
	language("PowerShell", "Windows や運用自動化で存在感がある .NET ベースのシェルです。", "runs_on_dotnet", "gc", "dollar_variables", "shell_scripting"),
	language("Perl", "テキスト処理やスクリプト用途で知られる、$ 変数の印象が強い言語です。", "gc", "dollar_variables"),
	language("Lua", "軽量で組み込み用途にもよく使われるスクリプト言語です。", "gc"),
	language("Objective-C", "Apple 系開発で長く使われてきた C 拡張ベースの言語です。", "compiled_native", "apple_ecosystem", "c_family_syntax", "oop_impression"),
	language("Clojure", "JVM 上で動く Lisp 系の関数型言語です。", "runs_on_jvm", "gc", "functional_first", "lisp_syntax"),
	language("F#", ".NET 上で動く関数型寄りの言語です。", "runs_on_dotnet", "gc", "functional_first"),
	language("R", "統計解析やデータ分析でよく使われる言語です。", "gc", "scientific_computing"),
	language("Julia", "高速な数値計算や科学技術計算で注目される言語です。", "gc", "compiled_native", "scientific_computing"),
	language("Nim", "Python 風の読みやすさとネイティブコンパイルを両立しやすい言語です。", "static_typing", "compiled_native", "indentation_sensitive"),
	language("Fortran", "科学技術計算の歴史でとても著名なコンパイル言語です。", "compiled_native", "scientific_computing"),
	language("COBOL", "業務システムの文脈で長い歴史を持つ著名な言語です。", "compiled_native"),
	language("Ada", "安全性を重視した静的型付きコンパイル言語です。", "static_typing", "compiled_native"),
	language("OCaml", "関数型寄りでネイティブコンパイルもできる言語です。", "static_typing", "compiled_native", "gc", "functional_first"),
	language("Erlang", "BEAM 上で動き、Actor モデルと並行処理で知られる言語です。", "gc", "functional_first", "actor_model", "lightweight_concurrency"),
	language("Common Lisp", "歴史ある Lisp 系の言語です。", "gc", "functional_first", "lisp_syntax"),
	language("Scheme", "教育や研究でもよく登場する Lisp 系言語です。", "gc", "functional_first", "lisp_syntax"),
	language("Visual Basic .NET", ".NET 上で動く Visual Basic 系の言語です。", "runs_on_dotnet", "gc", "oop_impression"),
	language("MATLAB", "数値計算やデータ解析の文脈で著名な言語です。", "scientific_computing"),
}

func main() {
	knowledgePath, err := defaultKnowledgePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	learned, err := loadLearnedLanguages(knowledgePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	game := Game{
		languages:     mergeLanguages(builtinLanguages, learned),
		learned:       learned,
		answers:       make(map[string]Answer),
		knowledgePath: knowledgePath,
	}

	if err := run(&game, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(game *Game, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "Guess The Lang")
	fmt.Fprintln(out, "Yes / No で答えてください。分からない場合は ? でスキップできます。")
	if len(game.learned) > 0 {
		fmt.Fprintf(out, "学習済みの追加言語: %d\n", len(game.learned))
	}
	fmt.Fprintln(out)

	for {
		remaining := remainingCandidates(game.languages, game.answers)

		switch len(remaining) {
		case 0:
			reportNoExactMatch(out, game.languages, game.answers)
			return offerLearning(reader, out, game)
		case 1:
			return finishSingleGuess(reader, out, game, remaining[0])
		}

		feature, ok := bestQuestion(remaining, game.answers)
		if !ok {
			reportAmbiguousResult(out, remaining)
			return offerLearning(reader, out, game)
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

func defaultKnowledgePath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("GUESS_THE_LANG_DATA")); override != "" {
		return override, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "guess-the-lang", "knowledge.json"), nil
}

func loadLearnedLanguages(path string) ([]Language, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var file knowledgeFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	languages := make([]Language, 0, len(file.Languages))
	for _, learned := range file.Languages {
		languages = append(languages, language(learned.Name, learned.Summary, learned.Features...))
	}

	return languages, nil
}

func saveLearnedLanguages(path string, learned []Language) error {
	entries := make([]persistedLanguage, 0, len(learned))
	for _, lang := range sortedLanguages(learned) {
		entries = append(entries, persistedLanguage{
			Name:     lang.Name,
			Summary:  lang.Summary,
			Features: sortedFeatureKeys(lang.Features),
		})
	}

	data, err := json.MarshalIndent(knowledgeFile{Languages: entries}, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func sortedLanguages(languages []Language) []Language {
	cloned := append([]Language(nil), languages...)
	sort.Slice(cloned, func(i, j int) bool {
		return strings.ToLower(cloned[i].Name) < strings.ToLower(cloned[j].Name)
	})
	return cloned
}

func sortedFeatureKeys(features map[string]bool) []string {
	keys := make([]string, 0, len(features))
	for key, enabled := range features {
		if enabled {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func mergeLanguages(base []Language, learned []Language) []Language {
	merged := make(map[string]Language, len(base)+len(learned))

	for _, lang := range base {
		merged[normalizeLanguageName(lang.Name)] = cloneLanguage(lang)
	}
	for _, lang := range learned {
		merged[normalizeLanguageName(lang.Name)] = cloneLanguage(lang)
	}

	result := make([]Language, 0, len(merged))
	for _, lang := range merged {
		result = append(result, lang)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result
}

func cloneLanguage(lang Language) Language {
	features := make(map[string]bool, len(lang.Features))
	for key, value := range lang.Features {
		features[key] = value
	}

	return Language{
		Name:     lang.Name,
		Summary:  lang.Summary,
		Features: features,
	}
}

func normalizeLanguageName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func finishSingleGuess(reader *bufio.Reader, out io.Writer, game *Game, candidate Language) error {
	reportSingleGuess(out, candidate, game.answers)

	correct, err := askYesNo(reader, out, "この言語で合っていますか？")
	if err != nil {
		if err == io.EOF {
			fmt.Fprintln(out)
			return nil
		}
		return err
	}

	if correct {
		fmt.Fprintln(out, "やった！")
		return nil
	}

	return learnLanguage(reader, out, game, candidate.Name)
}

func offerLearning(reader *bufio.Reader, out io.Writer, game *Game) error {
	teach, err := askYesNo(reader, out, "正解を学習させますか？")
	if err != nil {
		if err == io.EOF {
			fmt.Fprintln(out)
			return nil
		}
		return err
	}
	if !teach {
		return nil
	}

	return learnLanguage(reader, out, game, "")
}

func learnLanguage(reader *bufio.Reader, out io.Writer, game *Game, guessedName string) error {
	fmt.Fprintln(out)
	if guessedName != "" {
		fmt.Fprintf(out, "%s ではなかったのですね。学習させてください。\n", guessedName)
	} else {
		fmt.Fprintln(out, "正解の言語を学習させてください。")
	}

	name, err := askText(reader, out, "正解の言語名を教えてください", false)
	if err != nil {
		if err == io.EOF {
			fmt.Fprintln(out)
			return nil
		}
		return err
	}

	if hasBuiltinLanguage(name) {
		fmt.Fprintf(out, "%s は初期収録済みなので、新規学習としては保存しませんでした。\n", name)
		return nil
	}

	summary, err := askText(reader, out, "ひとこと説明を教えてください（空でも可）", true)
	stopAsking := false
	if err != nil {
		if err == io.EOF {
			stopAsking = true
		} else {
			return err
		}
	}

	learned := Language{
		Name:     name,
		Summary:  defaultSummary(name, summary),
		Features: make(map[string]bool),
	}

	for key, answer := range game.answers {
		if answer == Yes {
			learned.Features[key] = true
		}
	}

	if !stopAsking {
		for _, feature := range features {
			if _, alreadyKnown := game.answers[feature.Key]; alreadyKnown {
				continue
			}

			answer, err := ask(reader, out, Feature{Prompt: feature.Prompt + "（学習用）"})
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			if answer == Yes {
				learned.Features[feature.Key] = true
			}
		}
	}

	if err := rememberLearnedLanguage(game, learned); err != nil {
		return err
	}

	fmt.Fprintf(out, "学習しました: %s\n", learned.Name)
	fmt.Fprintf(out, "保存先: %s\n", game.knowledgePath)
	return nil
}

func rememberLearnedLanguage(game *Game, learned Language) error {
	game.learned = upsertLanguage(game.learned, learned)
	game.languages = mergeLanguages(builtinLanguages, game.learned)
	return saveLearnedLanguages(game.knowledgePath, game.learned)
}

func hasBuiltinLanguage(name string) bool {
	needle := normalizeLanguageName(name)
	for _, language := range builtinLanguages {
		if normalizeLanguageName(language.Name) == needle {
			return true
		}
	}
	return false
}

func upsertLanguage(languages []Language, candidate Language) []Language {
	key := normalizeLanguageName(candidate.Name)
	for i, language := range languages {
		if normalizeLanguageName(language.Name) == key {
			languages[i] = candidate
			return languages
		}
	}
	return append(languages, candidate)
}

func defaultSummary(name string, raw string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return fmt.Sprintf("%s の特徴を学習して追加された言語です。", name)
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

func askYesNo(reader *bufio.Reader, out io.Writer, prompt string) (bool, error) {
	for {
		answer, err := ask(reader, out, Feature{Prompt: prompt})
		if err != nil {
			return false, err
		}
		if answer == Unknown {
			fmt.Fprintln(out, "  y か n で答えてください。")
			continue
		}
		return answer == Yes, nil
	}
}

func askText(reader *bufio.Reader, out io.Writer, prompt string, allowEmpty bool) (string, error) {
	for {
		fmt.Fprintf(out, "- %s: ", prompt)
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if err != nil {
			if err == io.EOF {
				if trimmed == "" {
					if allowEmpty {
						return "", io.EOF
					}
					return "", io.EOF
				}
				return trimmed, nil
			}
			return "", err
		}

		if trimmed != "" || allowEmpty {
			return trimmed, nil
		}

		fmt.Fprintln(out, "  空ではなく入力してください。")
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
	fmt.Fprintln(out, "正解を学習させると、次回から候補に追加されます。")
}

func reportNoExactMatch(out io.Writer, candidates []Language, answers map[string]Answer) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "手元のデータでは完全一致が見つかりませんでした。近い候補は次のあたりです。")
	for _, match := range rankMatches(candidates, answers, 3) {
		fmt.Fprintf(out, "- %s (%d 一致 / %d 不一致): %s\n", match.Language.Name, match.Matches, match.Mismatches, match.Language.Summary)
	}
	fmt.Fprintln(out, "正解を学習させると、次回から候補に追加されます。")
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
