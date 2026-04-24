package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Answer int

const (
	Unknown Answer = iota
	Yes
	No
)

const builtInCatalogID = "programming-languages"

type Feature struct {
	Key    string
	Label  string
	Prompt string
}

type Question = Feature

type Language struct {
	Name     string
	Summary  string
	Features map[string]bool
}

type Entry = Language

type Catalog struct {
	ID        string
	Title     string
	Intro     string
	Questions []Question
	Entries   []Entry
}

type persistedLanguage struct {
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Features []string `json:"features"`
}

type persistedQuestion struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

type catalogFile struct {
	ID        string              `json:"id,omitempty"`
	Title     string              `json:"title"`
	Intro     string              `json:"intro"`
	Questions []persistedQuestion `json:"questions"`
	Entries   []persistedLanguage `json:"entries,omitempty"`
	Languages []persistedLanguage `json:"languages,omitempty"`
}

type knowledgeFile struct {
	Entries   []persistedLanguage `json:"entries,omitempty"`
	Languages []persistedLanguage `json:"languages,omitempty"`
}

type Match struct {
	Language   Language
	Matches    int
	Mismatches int
	Coverage   int
}

type randomizer interface {
	Intn(n int) int
}

type Game struct {
	catalog       Catalog
	entries       []Entry
	learned       []Entry
	answers       map[string]Answer
	knowledgePath string
	rng           randomizer
}

//go:embed catalogs/programming-languages.json
var defaultCatalogJSON []byte

var defaultCatalog = mustLoadBuiltInCatalog()

var features = defaultCatalog.Questions

var builtinLanguages = defaultCatalog.Entries

func main() {
	if err := runCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runCLI(args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) > 0 && args[0] == "catalog" {
		return runCatalogCommand(args[1:], out, errOut)
	}
	return runGuessCommand(args, in, out, errOut)
}

func runGuessCommand(args []string, in io.Reader, out, errOut io.Writer) error {
	flags := newFlagSet("guess-the-lang", errOut)
	catalogFlag := flags.String("catalog", "", "path to catalog JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}

	catalogPath := resolveCatalogPath(*catalogFlag)
	catalog, err := loadCatalog(catalogPath)
	if err != nil {
		return err
	}

	knowledgePath, err := defaultKnowledgePath(catalog, catalogPath)
	if err != nil {
		return err
	}

	learned, err := loadLearnedEntries(knowledgePath)
	if err != nil {
		return err
	}

	game := Game{
		catalog:       catalog,
		entries:       mergeEntries(catalog.Entries, learned),
		learned:       learned,
		answers:       make(map[string]Answer),
		knowledgePath: knowledgePath,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	return run(&game, in, out)
}

func runCatalogCommand(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: catalog <init|validate> ...")
	}

	switch args[0] {
	case "init":
		return runCatalogInit(args[1:], out, errOut)
	case "validate":
		return runCatalogValidate(args[1:], out, errOut)
	default:
		return fmt.Errorf("unknown catalog subcommand: %s", args[0])
	}
}

func runCatalogInit(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("catalog init", errOut)
	id := flags.String("id", "", "catalog id")
	title := flags.String("title", "", "catalog title")
	intro := flags.String("intro", "", "catalog intro")
	force := flags.Bool("force", false, "overwrite existing file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	path := strings.TrimSpace(flags.Arg(0))
	if path == "" {
		return errors.New("usage: catalog init [--id ID] [--title TITLE] [--intro INTRO] [--force] <path>")
	}

	if !*force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("catalog already exists: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	catalog := starterCatalog(path, *id, *title, *intro)
	if err := validateCatalog(catalog); err != nil {
		return err
	}
	if err := writeCatalog(path, catalog); err != nil {
		return err
	}

	fmt.Fprintf(out, "starter catalog written: %s\n", path)
	return nil
}

func runCatalogValidate(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("catalog validate", errOut)
	if err := flags.Parse(args); err != nil {
		return err
	}

	path := strings.TrimSpace(flags.Arg(0))
	if path == "" {
		return errors.New("usage: catalog validate <path>")
	}

	catalog, err := loadCatalog(path)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "catalog is valid: %s\n", path)
	if catalog.ID != "" {
		fmt.Fprintf(out, "id: %s\n", catalog.ID)
	}
	fmt.Fprintf(out, "questions: %d\n", len(catalog.Questions))
	fmt.Fprintf(out, "entries: %d\n", len(catalog.Entries))
	return nil
}

func newFlagSet(name string, errOut io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(errOut)
	return flags
}

func run(game *Game, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	if game.rng == nil {
		game.rng = rand.New(rand.NewSource(1))
	}

	fmt.Fprintln(out, game.catalog.Title)
	fmt.Fprintln(out, game.catalog.Intro)
	if len(game.learned) > 0 {
		fmt.Fprintf(out, "学習済みの追加項目: %d\n", len(game.learned))
	}
	fmt.Fprintln(out)

	for {
		remaining := remainingCandidates(game.entries, game.answers)

		switch len(remaining) {
		case 0:
			reportNoExactMatch(out, game.catalog.Questions, game.entries, game.answers)
			return offerLearning(reader, out, game)
		case 1:
			return finishSingleGuess(reader, out, game, remaining[0])
		}

		feature, ok := bestQuestionFrom(game.catalog.Questions, remaining, game.answers, game.rng)
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

func defaultKnowledgePath(catalog Catalog, catalogPath string) (string, error) {
	if override := strings.TrimSpace(os.Getenv("GUESS_THE_LANG_DATA")); override != "" {
		return override, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(catalog.ID) == builtInCatalogID {
		return filepath.Join(configDir, "guess-the-lang", "knowledge.json"), nil
	}

	if strings.TrimSpace(catalogPath) == "" {
		return filepath.Join(configDir, "guess-the-lang", "knowledge.json"), nil
	}

	if strings.TrimSpace(catalog.ID) != "" {
		return filepath.Join(configDir, "guess-the-lang", "knowledge-"+catalogIDStorageName(catalog.ID)+".json"), nil
	}

	if strings.TrimSpace(catalogPath) != "" {
		return filepath.Join(configDir, "guess-the-lang", "knowledge-"+catalogStorageName(catalogPath)+".json"), nil
	}

	return filepath.Join(configDir, "guess-the-lang", "knowledge.json"), nil
}

func catalogIDStorageName(id string) string {
	name := sanitizeStorageName(id)
	if name == "" {
		return "custom"
	}
	return name + "-" + shortTextHash(strings.TrimSpace(id))
}

func catalogStorageName(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name := sanitizeStorageName(base)
	if name == "" {
		name = "custom"
	}
	return name + "-" + shortPathHash(path)
}

func sanitizeStorageName(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func shortPathHash(path string) string {
	resolved := path
	if abs, err := filepath.Abs(path); err == nil {
		resolved = abs
	}
	return shortTextHash(filepath.Clean(resolved))
}

func shortTextHash(value string) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(value))
	return fmt.Sprintf("%08x", hasher.Sum32())
}

func resolveCatalogPath(flagValue string) string {
	if override := strings.TrimSpace(flagValue); override != "" {
		return override
	}
	return strings.TrimSpace(os.Getenv("GUESS_THE_LANG_CATALOG"))
}

func mustLoadBuiltInCatalog() Catalog {
	catalog, err := loadCatalogBytes(defaultCatalogJSON)
	if err != nil {
		panic(err)
	}
	return catalog
}

func loadCatalog(path string) (Catalog, error) {
	if strings.TrimSpace(path) == "" {
		return defaultCatalog, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	return loadCatalogBytes(data)
}

func loadCatalogBytes(data []byte) (Catalog, error) {
	var file catalogFile
	if err := json.Unmarshal(data, &file); err != nil {
		return Catalog{}, err
	}

	catalog := catalogFromFile(file)
	if err := validateCatalog(catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func catalogFromFile(file catalogFile) Catalog {
	questions := make([]Question, 0, len(file.Questions))
	for _, question := range file.Questions {
		questions = append(questions, Question{
			Key:    strings.TrimSpace(question.Key),
			Label:  strings.TrimSpace(question.Label),
			Prompt: strings.TrimSpace(question.Prompt),
		})
	}

	entries := file.Entries
	if len(entries) == 0 {
		entries = file.Languages
	}

	loadedEntries := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		loadedEntries = append(loadedEntries, language(strings.TrimSpace(entry.Name), strings.TrimSpace(entry.Summary), normalizeFeatureKeys(entry.Features)...))
	}

	return Catalog{
		ID:        strings.TrimSpace(file.ID),
		Title:     strings.TrimSpace(file.Title),
		Intro:     strings.TrimSpace(file.Intro),
		Questions: questions,
		Entries:   loadedEntries,
	}
}

func normalizeFeatureKeys(keys []string) []string {
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func validateCatalog(catalog Catalog) error {
	if catalog.ID != "" && sanitizeStorageName(catalog.ID) == "" {
		return errors.New("catalog id must contain at least one letter or number")
	}
	if strings.TrimSpace(catalog.Title) == "" {
		return errors.New("catalog title must not be empty")
	}
	if strings.TrimSpace(catalog.Intro) == "" {
		return errors.New("catalog intro must not be empty")
	}
	if len(catalog.Questions) == 0 {
		return errors.New("catalog must contain at least one question")
	}
	if len(catalog.Entries) == 0 {
		return errors.New("catalog must contain at least one entry")
	}

	knownQuestions := make(map[string]struct{}, len(catalog.Questions))
	for _, question := range catalog.Questions {
		key := strings.TrimSpace(question.Key)
		if key == "" {
			return errors.New("catalog question key must not be empty")
		}
		if strings.TrimSpace(question.Label) == "" {
			return fmt.Errorf("catalog question %s label must not be empty", key)
		}
		if strings.TrimSpace(question.Prompt) == "" {
			return fmt.Errorf("catalog question %s prompt must not be empty", key)
		}
		if _, exists := knownQuestions[key]; exists {
			return fmt.Errorf("duplicate catalog question key: %s", key)
		}
		knownQuestions[key] = struct{}{}
	}

	knownEntries := make(map[string]struct{}, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		name := normalizeName(entry.Name)
		if name == "" {
			return errors.New("catalog entry name must not be empty")
		}
		if _, exists := knownEntries[name]; exists {
			return fmt.Errorf("duplicate catalog entry: %s", entry.Name)
		}
		knownEntries[name] = struct{}{}

		for key := range entry.Features {
			if _, ok := knownQuestions[key]; !ok {
				return fmt.Errorf("catalog entry %s uses unknown feature %s", entry.Name, key)
			}
		}
	}

	return nil
}

func starterCatalog(path string, id, title, intro string) Catalog {
	derivedID := strings.TrimSpace(id)
	if derivedID == "" {
		derivedID = sanitizeStorageName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	if derivedID == "" {
		derivedID = "new-catalog"
	}

	derivedTitle := strings.TrimSpace(title)
	if derivedTitle == "" {
		derivedTitle = "New Catalog"
	}

	derivedIntro := strings.TrimSpace(intro)
	if derivedIntro == "" {
		derivedIntro = "Yes / No で答えてください。分からない場合は ? でスキップできます。"
	}

	return Catalog{
		ID:    derivedID,
		Title: derivedTitle,
		Intro: derivedIntro,
		Questions: []Question{
			{Key: "feature_a", Label: "特徴A", Prompt: "特徴Aですか？"},
			{Key: "feature_b", Label: "特徴B", Prompt: "特徴Bですか？"},
		},
		Entries: []Entry{
			language("Item A", "最初の候補です。", "feature_a"),
			language("Item B", "2 番目の候補です。", "feature_b"),
		},
	}
}

func writeCatalog(path string, catalog Catalog) error {
	file := catalogToFile(catalog)
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func catalogToFile(catalog Catalog) catalogFile {
	questions := make([]persistedQuestion, 0, len(catalog.Questions))
	for _, question := range catalog.Questions {
		questions = append(questions, persistedQuestion{
			Key:    question.Key,
			Label:  question.Label,
			Prompt: question.Prompt,
		})
	}

	entries := make([]persistedLanguage, 0, len(catalog.Entries))
	for _, entry := range sortedEntries(catalog.Entries) {
		entries = append(entries, persistedLanguage{
			Name:     entry.Name,
			Summary:  entry.Summary,
			Features: sortedFeatureKeys(entry.Features),
		})
	}

	return catalogFile{
		ID:        catalog.ID,
		Title:     catalog.Title,
		Intro:     catalog.Intro,
		Questions: questions,
		Entries:   entries,
	}
}

func loadLearnedEntries(path string) ([]Entry, error) {
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
	entries := file.Entries
	if len(entries) == 0 {
		entries = file.Languages
	}

	learnedEntries := make([]Entry, 0, len(entries))
	for _, learned := range entries {
		learnedEntries = append(learnedEntries, language(learned.Name, learned.Summary, learned.Features...))
	}

	return learnedEntries, nil
}

func loadLearnedLanguages(path string) ([]Language, error) {
	return loadLearnedEntries(path)
}

func saveLearnedEntries(path string, learned []Entry) error {
	entries := make([]persistedLanguage, 0, len(learned))
	for _, lang := range sortedEntries(learned) {
		entries = append(entries, persistedLanguage{
			Name:     lang.Name,
			Summary:  lang.Summary,
			Features: sortedFeatureKeys(lang.Features),
		})
	}

	data, err := json.MarshalIndent(knowledgeFile{Entries: entries, Languages: entries}, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func saveLearnedLanguages(path string, learned []Language) error {
	return saveLearnedEntries(path, learned)
}

func sortedEntries(entries []Entry) []Entry {
	cloned := append([]Entry(nil), entries...)
	sort.Slice(cloned, func(i, j int) bool {
		return strings.ToLower(cloned[i].Name) < strings.ToLower(cloned[j].Name)
	})
	return cloned
}

func sortedLanguages(languages []Language) []Language {
	return sortedEntries(languages)
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

func mergeEntries(base []Entry, learned []Entry) []Entry {
	merged := make(map[string]Entry, len(base)+len(learned))

	for _, lang := range base {
		merged[normalizeName(lang.Name)] = cloneEntry(lang)
	}
	for _, lang := range learned {
		merged[normalizeName(lang.Name)] = cloneEntry(lang)
	}

	result := make([]Entry, 0, len(merged))
	for _, lang := range merged {
		result = append(result, lang)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result
}

func mergeLanguages(base []Language, learned []Language) []Language {
	return mergeEntries(base, learned)
}

func cloneEntry(lang Entry) Entry {
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

func cloneLanguage(lang Language) Language {
	return cloneEntry(lang)
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeLanguageName(name string) string {
	return normalizeName(name)
}

func finishSingleGuess(reader *bufio.Reader, out io.Writer, game *Game, candidate Language) error {
	reportSingleGuess(out, game.catalog.Questions, candidate, game.answers)

	correct, err := askYesNo(reader, out, "この候補で合っていますか？")
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
		fmt.Fprintln(out, "正解の項目を学習させてください。")
	}

	name, err := askText(reader, out, "正解の名前を教えてください", false)
	if err != nil {
		if err == io.EOF {
			fmt.Fprintln(out)
			return nil
		}
		return err
	}

	if hasCatalogEntry(game.catalog.Entries, name) {
		fmt.Fprintf(out, "%s は初期データに含まれているので、新規学習としては保存しませんでした。\n", name)
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
		for _, feature := range game.catalog.Questions {
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
	game.learned = upsertEntry(game.learned, learned)
	game.entries = mergeEntries(game.catalog.Entries, game.learned)
	return saveLearnedEntries(game.knowledgePath, game.learned)
}

func hasCatalogEntry(entries []Entry, name string) bool {
	needle := normalizeName(name)
	for _, entry := range entries {
		if normalizeName(entry.Name) == needle {
			return true
		}
	}
	return false
}

func hasBuiltinLanguage(name string) bool {
	return hasCatalogEntry(defaultCatalog.Entries, name)
}

func upsertEntry(entries []Entry, candidate Entry) []Entry {
	key := normalizeName(candidate.Name)
	for i, entry := range entries {
		if normalizeName(entry.Name) == key {
			entries[i] = candidate
			return entries
		}
	}
	return append(entries, candidate)
}

func upsertLanguage(languages []Language, candidate Language) []Language {
	return upsertEntry(languages, candidate)
}

func defaultSummary(name string, raw string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return fmt.Sprintf("%s の特徴を学習して追加された項目です。", name)
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

func bestQuestionFrom(questions []Question, candidates []Entry, answers map[string]Answer, rng randomizer) (Question, bool) {
	bestScore := -1
	bestFeatures := make([]Question, 0, 4)

	for _, feature := range questions {
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
			bestFeatures = []Feature{feature}
		} else if score == bestScore {
			bestFeatures = append(bestFeatures, feature)
		}
	}

	if bestScore < 0 || len(bestFeatures) == 0 {
		return Question{}, false
	}
	if rng == nil || len(bestFeatures) == 1 {
		return bestFeatures[0], true
	}
	return bestFeatures[rng.Intn(len(bestFeatures))], true
}

func reportSingleGuess(out io.Writer, questions []Question, candidate Entry, answers map[string]Answer) {
	fmt.Fprintln(out)
	fmt.Fprintf(out, "たぶん %s です。\n", candidate.Name)
	if highlights := matchingHighlights(questions, candidate, answers, 4); len(highlights) > 0 {
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

func reportNoExactMatch(out io.Writer, questions []Question, candidates []Entry, answers map[string]Answer) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "手元のデータでは完全一致が見つかりませんでした。近い候補は次のあたりです。")
	for _, match := range rankMatches(candidates, answers, 3) {
		fmt.Fprintf(out, "- %s (%d/%d 一致, %d 不一致): %s\n", match.Language.Name, match.Matches, match.Coverage, match.Mismatches, match.Language.Summary)
		if reasons := matchReasons(questions, match.Language, answers, 4); len(reasons) > 0 {
			fmt.Fprintf(out, "  一致点: %s\n", strings.Join(reasons, " / "))
		}
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

func matchingHighlights(questions []Question, candidate Entry, answers map[string]Answer, limit int) []string {
	labels := make([]string, 0, limit)
	for _, feature := range questions {
		if len(labels) >= limit {
			break
		}
		if answers[feature.Key] == Yes && candidate.Features[feature.Key] {
			labels = append(labels, feature.Label)
		}
	}
	return labels
}

func matchReasons(questions []Question, candidate Entry, answers map[string]Answer, limit int) []string {
	reasons := make([]string, 0, limit)
	for _, feature := range questions {
		if len(reasons) >= limit {
			break
		}

		switch answers[feature.Key] {
		case Yes:
			if candidate.Features[feature.Key] {
				reasons = append(reasons, feature.Label)
			}
		case No:
			if !candidate.Features[feature.Key] {
				reasons = append(reasons, feature.Label+"ではない")
			}
		}
	}
	return reasons
}
