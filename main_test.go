package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubRandomizer struct {
	value int
}

func (s stubRandomizer) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return s.value % n
}

func testCatalog(entries []Language) Catalog {
	return Catalog{
		Title:     "Test Catalog",
		Intro:     "Test intro",
		Questions: features,
		Entries:   entries,
	}
}

func testGame(entries []Language, answers map[string]Answer, knowledgePath string) Game {
	return testGameWithCatalog(entries, entries, answers, knowledgePath)
}

func testGameWithCatalog(catalogEntries []Language, entries []Language, answers map[string]Answer, knowledgePath string) Game {
	return Game{
		catalog:       testCatalog(catalogEntries),
		entries:       entries,
		answers:       answers,
		knowledgePath: knowledgePath,
	}
}

func writeCatalogFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}
	return path
}

func TestRunCatalogInitWritesValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "starter.json")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runCatalogCommand([]string{"init", "--id", "animals", "--title", "Guess The Animal", path}, &out, &errOut)
	if err != nil {
		t.Fatalf("runCatalogCommand returned error: %v", err)
	}
	if !strings.Contains(out.String(), "starter catalog written") {
		t.Fatalf("expected init confirmation, got %q", out.String())
	}

	catalog, err := loadCatalog(path)
	if err != nil {
		t.Fatalf("loadCatalog returned error: %v", err)
	}
	if catalog.ID != "animals" || catalog.Title != "Guess The Animal" {
		t.Fatalf("unexpected starter catalog: %+v", catalog)
	}
	if len(catalog.Questions) == 0 || len(catalog.Entries) == 0 {
		t.Fatalf("expected starter catalog content, got %+v", catalog)
	}
}

func TestRunCatalogInitDoesNotOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "starter.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	err := runCatalogCommand([]string{"init", path}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing file error, got %v", err)
	}
}

func TestRunCatalogValidateReportsSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := writeCatalog(path, starterCatalog(path, "animals", "Guess The Animal", "intro")); err != nil {
		t.Fatalf("writeCatalog returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := runCatalogCommand([]string{"validate", path}, &out, &errOut)
	if err != nil {
		t.Fatalf("runCatalogCommand returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "catalog is valid") || !strings.Contains(got, "questions: 2") || !strings.Contains(got, "entries: 2") {
		t.Fatalf("expected validation summary, got %q", got)
	}
}

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

	question, ok := bestQuestionFrom(defaultCatalog.Questions, candidates, answers, nil)
	if !ok {
		t.Fatal("expected a discriminating question")
	}

	if question.Key != "oop_impression" {
		t.Fatalf("expected oop_impression, got %s", question.Key)
	}
}

func TestBestQuestionCanRandomizeAmongEqualScores(t *testing.T) {
	candidates := []Language{
		language("LangA", "", "static_typing", "gc"),
		language("LangB", "", "static_typing", "web_backend"),
		language("LangC", "", "gc", "web_backend"),
	}

	questionA, ok := bestQuestionFrom(defaultCatalog.Questions, candidates, map[string]Answer{}, stubRandomizer{value: 0})
	if !ok {
		t.Fatal("expected a discriminating question")
	}

	questionB, ok := bestQuestionFrom(defaultCatalog.Questions, candidates, map[string]Answer{}, stubRandomizer{value: 1})
	if !ok {
		t.Fatal("expected a discriminating question")
	}

	if questionA.Key == questionB.Key {
		t.Fatalf("expected different equally strong questions, got %s and %s", questionA.Key, questionB.Key)
	}

	allowed := map[string]bool{
		"static_typing": true,
		"gc":            true,
		"web_backend":   true,
	}
	if !allowed[questionA.Key] || !allowed[questionB.Key] {
		t.Fatalf("expected tied top questions, got %s and %s", questionA.Key, questionB.Key)
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

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(raw), "\"entries\"") || !strings.Contains(string(raw), "\"languages\"") {
		t.Fatalf("expected saved knowledge to include both entries and legacy languages keys, got %q", string(raw))
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

func TestLoadLearnedLanguagesSupportsLegacyLanguagesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.json")
	data := []byte("{\n  \"languages\": [\n    {\n      \"name\": \"Zig\",\n      \"summary\": \"A systems language.\",\n      \"features\": [\"static_typing\", \"compiled_native\"]\n    }\n  ]\n}\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	loaded, err := loadLearnedEntries(path)
	if err != nil {
		t.Fatalf("loadLearnedEntries returned error: %v", err)
	}

	if len(loaded) != 1 || loaded[0].Name != "Zig" {
		t.Fatalf("unexpected loaded entries: %+v", loaded)
	}
	if !loaded[0].Features["static_typing"] || !loaded[0].Features["compiled_native"] {
		t.Fatalf("expected legacy features to round-trip, got %+v", loaded[0].Features)
	}
}

func TestResolveCatalogPathPrefersFlagOverEnv(t *testing.T) {
	t.Setenv("GUESS_THE_LANG_CATALOG", "/tmp/from-env.json")

	got := resolveCatalogPath("/tmp/from-flag.json")
	if got != "/tmp/from-flag.json" {
		t.Fatalf("expected flag value to win, got %q", got)
	}
}

func TestResolveCatalogPathFallsBackToEnv(t *testing.T) {
	t.Setenv("GUESS_THE_LANG_CATALOG", "/tmp/from-env.json")

	got := resolveCatalogPath("")
	if got != "/tmp/from-env.json" {
		t.Fatalf("expected env value fallback, got %q", got)
	}
}

func TestLoadCatalogFromJSONFile(t *testing.T) {
	path := writeCatalogFile(t, `{
	  "id": "animals",
	  "title": "Guess The Animal",
	  "intro": "動物について yes / no で答えてください。",
	  "questions": [
	    {"key": "barks", "label": "吠える", "prompt": "吠えますか？"},
	    {"key": "meows", "label": "鳴く", "prompt": "よく鳴きますか？"}
	  ],
	  "entries": [
	    {"name": "Dog", "summary": "Barks loudly.", "features": ["barks"]},
	    {"name": "Cat", "summary": "Often meows.", "features": ["meows"]}
	  ]
	}`)

	catalog, err := loadCatalog(path)
	if err != nil {
		t.Fatalf("loadCatalog returned error: %v", err)
	}

	if catalog.Title != "Guess The Animal" {
		t.Fatalf("unexpected catalog title: %q", catalog.Title)
	}
	if catalog.ID != "animals" {
		t.Fatalf("unexpected catalog id: %q", catalog.ID)
	}
	if len(catalog.Questions) != 2 || catalog.Questions[0].Prompt != "吠えますか？" {
		t.Fatalf("unexpected loaded questions: %+v", catalog.Questions)
	}
	if len(catalog.Entries) != 2 || catalog.Entries[0].Name != "Dog" {
		t.Fatalf("unexpected loaded entries: %+v", catalog.Entries)
	}
}

func TestLoadCatalogRejectsUnknownFeatures(t *testing.T) {
	path := writeCatalogFile(t, `{
	  "title": "Broken Catalog",
	  "intro": "broken",
	  "questions": [
	    {"key": "barks", "label": "吠える", "prompt": "吠えますか？"}
	  ],
	  "entries": [
	    {"name": "Dog", "summary": "Barks loudly.", "features": ["unknown"]}
	  ]
	}`)

	_, err := loadCatalog(path)
	if err == nil || !strings.Contains(err.Error(), "unknown feature") {
		t.Fatalf("expected unknown feature validation error, got %v", err)
	}
}

func TestLoadCatalogRejectsInvalidID(t *testing.T) {
	path := writeCatalogFile(t, `{
	  "id": "---",
	  "title": "Broken Catalog",
	  "intro": "broken",
	  "questions": [
	    {"key": "barks", "label": "吠える", "prompt": "吠えますか？"}
	  ],
	  "entries": [
	    {"name": "Dog", "summary": "Barks loudly.", "features": ["barks"]}
	  ]
	}`)

	_, err := loadCatalog(path)
	if err == nil || !strings.Contains(err.Error(), "catalog id") {
		t.Fatalf("expected catalog id validation error, got %v", err)
	}
}

func TestLoadCatalogRejectsBlankRequiredFields(t *testing.T) {
	path := writeCatalogFile(t, `{
	  "title": "",
	  "intro": "intro",
	  "questions": [
	    {"key": "barks", "label": "", "prompt": "吠えますか？"}
	  ],
	  "entries": [
	    {"name": "Dog", "summary": "Barks loudly.", "features": ["barks"]}
	  ]
	}`)

	_, err := loadCatalog(path)
	if err == nil || (!strings.Contains(err.Error(), "title") && !strings.Contains(err.Error(), "label")) {
		t.Fatalf("expected required field validation error, got %v", err)
	}
}

func TestLoadCatalogTrimsQuestionAndFeatureKeys(t *testing.T) {
	path := writeCatalogFile(t, `{
	  "title": " Guess The Animal ",
	  "intro": " 動物について yes / no で答えてください。 ",
	  "questions": [
	    {"key": " barks ", "label": " 吠える ", "prompt": " 吠えますか？ "}
	  ],
	  "entries": [
	    {"name": " Dog ", "summary": " Barks loudly. ", "features": [" barks "]}
	  ]
	}`)

	catalog, err := loadCatalog(path)
	if err != nil {
		t.Fatalf("loadCatalog returned error: %v", err)
	}

	if catalog.Title != "Guess The Animal" || catalog.Intro != "動物について yes / no で答えてください。" {
		t.Fatalf("expected trimmed catalog metadata, got %+v", catalog)
	}
	if catalog.Questions[0].Key != "barks" || catalog.Questions[0].Prompt != "吠えますか？" {
		t.Fatalf("expected trimmed question, got %+v", catalog.Questions[0])
	}
	if catalog.Entries[0].Name != "Dog" || !catalog.Entries[0].Features["barks"] {
		t.Fatalf("expected trimmed entry, got %+v", catalog.Entries[0])
	}
}

func TestDefaultKnowledgePathUsesSeparateFileForCustomCatalog(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got, err := defaultKnowledgePath(Catalog{}, "/tmp/Guess The Animal.json")
	if err != nil {
		t.Fatalf("defaultKnowledgePath returned error: %v", err)
	}

	if !strings.Contains(got, filepath.Join("guess-the-lang", "knowledge-guess-the-animal-")) || !strings.HasSuffix(got, ".json") {
		t.Fatalf("expected custom catalog-specific knowledge path, got %q", got)
	}
}

func TestDefaultKnowledgePathUsesCatalogIDWhenPresent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	left, err := defaultKnowledgePath(Catalog{ID: "animals"}, "/tmp/animals.json")
	if err != nil {
		t.Fatalf("defaultKnowledgePath returned error: %v", err)
	}
	right, err := defaultKnowledgePath(Catalog{ID: "animals"}, "/some/other/place/animals-copy.json")
	if err != nil {
		t.Fatalf("defaultKnowledgePath returned error: %v", err)
	}

	if left != right {
		t.Fatalf("expected same knowledge path for same catalog id, got %q and %q", left, right)
	}
	if !strings.Contains(left, filepath.Join("guess-the-lang", "knowledge-animals-")) || !strings.HasSuffix(left, ".json") {
		t.Fatalf("expected id-based knowledge path, got %q", left)
	}
}

func TestCatalogIDStorageNameAvoidsSanitizationCollisions(t *testing.T) {
	left := catalogIDStorageName("foo/bar")
	right := catalogIDStorageName("foo bar")
	if left == right {
		t.Fatalf("expected different storage names for colliding sanitized ids, got %q and %q", left, right)
	}
}

func TestDefaultKnowledgePathKeepsBuiltinCatalogContinuity(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	fromEmbedded, err := defaultKnowledgePath(Catalog{ID: builtInCatalogID}, "")
	if err != nil {
		t.Fatalf("defaultKnowledgePath returned error: %v", err)
	}
	fromFile, err := defaultKnowledgePath(Catalog{ID: builtInCatalogID}, filepath.Join("catalogs", "programming-languages.json"))
	if err != nil {
		t.Fatalf("defaultKnowledgePath returned error: %v", err)
	}

	if fromEmbedded != fromFile {
		t.Fatalf("expected built-in and file catalog paths to match, got %q and %q", fromEmbedded, fromFile)
	}
	if !strings.HasSuffix(fromEmbedded, filepath.Join("guess-the-lang", "knowledge.json")) {
		t.Fatalf("expected builtin catalog to keep legacy knowledge path, got %q", fromEmbedded)
	}
}

func TestCatalogStorageNameIncludesPathHash(t *testing.T) {
	left := catalogStorageName("/a/catalog.json")
	right := catalogStorageName("/b/catalog.json")
	if left == right {
		t.Fatalf("expected different storage names for same basename, got %q and %q", left, right)
	}
}

func TestRunWorksWithCustomCatalogContent(t *testing.T) {
	entries := []Language{
		language("Dog", "Barks loudly.", "barks"),
		language("Cat", "Often meows.", "meows"),
	}
	game := Game{
		catalog: Catalog{
			Title:     "Guess The Animal",
			Intro:     "動物について yes / no で答えてください。",
			Questions: []Question{{Key: "barks", Label: "吠える", Prompt: "吠えますか？"}, {Key: "meows", Label: "鳴く", Prompt: "よく鳴きますか？"}},
			Entries:   entries,
		},
		entries:       entries,
		answers:       map[string]Answer{},
		knowledgePath: filepath.Join(t.TempDir(), "knowledge.json"),
		rng:           stubRandomizer{value: 0},
	}

	var out bytes.Buffer
	err := run(&game, strings.NewReader("y\ny\n"), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Guess The Animal") {
		t.Fatalf("expected custom title, got %q", got)
	}
	if !strings.Contains(got, "- 吠えますか？ [y/n/?]:") {
		t.Fatalf("expected custom catalog question prompt, got %q", got)
	}
	if !strings.Contains(got, "たぶん Dog です。") {
		t.Fatalf("expected custom catalog entry guess, got %q", got)
	}
}

func TestSampleCatalogFilesLoad(t *testing.T) {
	tests := []struct {
		path      string
		wantTitle string
	}{
		{path: filepath.Join("catalogs", "programming-languages.json"), wantTitle: "Guess The Lang"},
		{path: filepath.Join("catalogs", "animals.json"), wantTitle: "Guess The Animal"},
		{path: filepath.Join("catalogs", "fruits.json"), wantTitle: "Guess The Fruit"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			catalog, err := loadCatalog(tc.path)
			if err != nil {
				t.Fatalf("loadCatalog returned error: %v", err)
			}
			if catalog.Title != tc.wantTitle {
				t.Fatalf("unexpected catalog title: %q", catalog.Title)
			}
			if catalog.ID == "" {
				t.Fatalf("expected sample catalog to define an id: %+v", catalog)
			}
			if len(catalog.Questions) == 0 || len(catalog.Entries) == 0 {
				t.Fatalf("expected sample catalog to contain questions and entries: %+v", catalog)
			}
		})
	}
}

func TestEmbeddedDefaultCatalogMatchesProgrammingLanguagesFile(t *testing.T) {
	fromFile, err := loadCatalog(filepath.Join("catalogs", "programming-languages.json"))
	if err != nil {
		t.Fatalf("loadCatalog returned error: %v", err)
	}

	if defaultCatalog.Title != fromFile.Title || defaultCatalog.Intro != fromFile.Intro {
		t.Fatalf("expected embedded default catalog metadata to match file: %+v vs %+v", defaultCatalog, fromFile)
	}
	if len(defaultCatalog.Questions) != len(fromFile.Questions) || len(defaultCatalog.Entries) != len(fromFile.Entries) {
		t.Fatalf("expected embedded default catalog sizes to match file: %+v vs %+v", defaultCatalog, fromFile)
	}
}

func TestRunWorksWithSampleAnimalsCatalog(t *testing.T) {
	catalog, err := loadCatalog(filepath.Join("catalogs", "animals.json"))
	if err != nil {
		t.Fatalf("loadCatalog returned error: %v", err)
	}

	game := Game{
		catalog:       catalog,
		entries:       catalog.Entries,
		answers:       map[string]Answer{},
		knowledgePath: filepath.Join(t.TempDir(), "knowledge.json"),
		rng:           stubRandomizer{value: 0},
	}

	var out bytes.Buffer
	err = run(&game, strings.NewReader("y\ny\n"), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Guess The Animal") {
		t.Fatalf("expected sample catalog title, got %q", got)
	}
	if !strings.Contains(got, "たぶん Dog です。") {
		t.Fatalf("expected sample catalog guess, got %q", got)
	}
}

func TestRunRetriesInvalidInputAndFindsCPP(t *testing.T) {
	game := testGame([]Language{
		language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
		language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	}, map[string]Answer{}, filepath.Join(t.TempDir(), "knowledge.json"))

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
	game := testGame([]Language{
		language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
		language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	}, map[string]Answer{}, filepath.Join(t.TempDir(), "knowledge.json"))

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
	game := testGame([]Language{
		language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
		language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	}, map[string]Answer{
		"manual_memory": No,
	}, filepath.Join(t.TempDir(), "knowledge.json"))

	var out bytes.Buffer
	err := run(&game, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "完全一致が見つかりませんでした") {
		t.Fatalf("expected no exact match message, got %q", out.String())
	}
}

func TestRunReportsWhyCandidatesAreCloseWhenNoExactMatch(t *testing.T) {
	game := testGame([]Language{
		language("C", "Low-level language.", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
		language("C++", "OOP-capable systems language.", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	}, map[string]Answer{
		"static_typing":   Yes,
		"compiled_native": Yes,
		"manual_memory":   Yes,
		"web_backend":     Yes,
	}, filepath.Join(t.TempDir(), "knowledge.json"))

	var out bytes.Buffer
	err := run(&game, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "3/4 一致") {
		t.Fatalf("expected match coverage in no-exact-match output, got %q", got)
	}
	if !strings.Contains(got, "- C (3/4 一致, 1 不一致): Low-level language.") {
		t.Fatalf("expected C to appear in ranked candidates, got %q", got)
	}
	if !strings.Contains(got, "- C++ (3/4 一致, 1 不一致): OOP-capable systems language.") {
		t.Fatalf("expected C++ to appear in ranked candidates, got %q", got)
	}
	if !strings.Contains(got, "一致点: 静的型付け / ネイティブコンパイル / 手動メモリ管理") {
		t.Fatalf("expected matching feature highlights in no-exact-match output, got %q", got)
	}
	if strings.Index(got, "- C (3/4 一致, 1 不一致): Low-level language.") > strings.Index(got, "- C++ (3/4 一致, 1 不一致): OOP-capable systems language.") {
		t.Fatalf("expected ranked candidates to stay name-sorted for tied scores, got %q", got)
	}
}

func TestRunReportsNegativeMatchReasonsWhenNoExactMatch(t *testing.T) {
	game := testGame([]Language{
		language("Go", "Concurrent language.", "static_typing", "compiled_native", "gc", "web_backend", "c_family_syntax", "lightweight_concurrency"),
		language("Rust", "Safe systems language.", "static_typing", "compiled_native", "ownership_model", "c_family_syntax"),
	}, map[string]Answer{
		"manual_memory":  No,
		"browser_native": Yes,
	}, filepath.Join(t.TempDir(), "knowledge.json"))

	var out bytes.Buffer
	err := run(&game, strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "一致点: 手動メモリ管理ではない") {
		t.Fatalf("expected negative match reason in no-exact-match output, got %q", got)
	}
}

func TestRunLearnsLanguageOnWrongGuess(t *testing.T) {
	knowledgePath := filepath.Join(t.TempDir(), "knowledge.json")
	game := testGame([]Language{
		language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
		language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
	}, map[string]Answer{}, knowledgePath)

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
	game := testGameWithCatalog(
		[]Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
			language("Go", "", "static_typing", "compiled_native", "gc", "web_backend", "c_family_syntax", "lightweight_concurrency"),
		},
		[]Language{
			language("C", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax"),
			language("C++", "", "static_typing", "compiled_native", "manual_memory", "c_family_syntax", "oop_impression"),
		},
		map[string]Answer{},
		knowledgePath,
	)

	var out bytes.Buffer
	input := "n\nn\nGo\n"
	err := run(&game, strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "初期データに含まれている") {
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

func TestBuiltinLanguagesStayConsistent(t *testing.T) {
	knownFeatures := make(map[string]struct{}, len(features))
	for _, feature := range features {
		if _, exists := knownFeatures[feature.Key]; exists {
			t.Fatalf("duplicate feature key: %s", feature.Key)
		}
		knownFeatures[feature.Key] = struct{}{}
	}

	seenLanguages := make(map[string]struct{}, len(builtinLanguages))
	for _, lang := range builtinLanguages {
		normalized := normalizeLanguageName(lang.Name)
		if _, exists := seenLanguages[normalized]; exists {
			t.Fatalf("duplicate builtin language: %s", lang.Name)
		}
		seenLanguages[normalized] = struct{}{}

		for key := range lang.Features {
			if _, ok := knownFeatures[key]; !ok {
				t.Fatalf("language %s uses unknown feature %s", lang.Name, key)
			}
		}
	}
}

func TestBuiltinLanguageCatalogIncludesNewFeatureFamilies(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{name: "APL", want: []string{"array_programming", "scientific_computing"}},
		{name: "BASIC", want: []string{"basic_family", "education_use"}},
		{name: "PostScript", want: []string{"stack_based"}},
		{name: "Prolog", want: []string{"logic_programming"}},
		{name: "Visual Basic .NET", want: []string{"runs_on_dotnet", "basic_family"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got Language
			found := false
			for _, lang := range builtinLanguages {
				if lang.Name == tc.name {
					got = lang
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("builtin language %s not found", tc.name)
			}

			for _, feature := range tc.want {
				if !got.Features[feature] {
					t.Fatalf("expected %s to have feature %s", tc.name, feature)
				}
			}
		})
	}
}

func TestTypeScriptIsNotTreatedAsBrowserNative(t *testing.T) {
	answers := map[string]Answer{
		"superset_of_js": Yes,
		"browser_native": Yes,
	}

	got := remainingCandidates(builtinLanguages, answers)
	for _, candidate := range got {
		if candidate.Name == "TypeScript" {
			t.Fatal("expected TypeScript to be excluded when browser_native=yes")
		}
	}
}
