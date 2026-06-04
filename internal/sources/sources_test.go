package sources

import "testing"

func TestSplitAnswerAndSourcesExtractsBodyMarkdownLinks(t *testing.T) {
	text := "OpenAI Responses API 的 `web_search` 工具通过 `tools` 启用。([platform.openai.com](https://platform.openai.com/docs/guides/tools-web-search?api-mode=responses&utm_source=openai))\n\n```bash\ncurl https://api.openai.com/v1/responses\n```"

	answer, sources := SplitAnswerAndSources(text)

	if answer != text {
		t.Fatalf("answer should stay unchanged:\n%q", answer)
	}
	if len(sources) != 1 {
		t.Fatalf("sources = %#v", sources)
	}
	if sources[0]["title"] != "platform.openai.com" {
		t.Fatalf("source title = %#v", sources[0])
	}
	if sources[0]["url"] != "https://platform.openai.com/docs/guides/tools-web-search?api-mode=responses&utm_source=openai" {
		t.Fatalf("source url = %#v", sources[0])
	}
}

func TestSplitAnswerAndSourcesKeepsAnnotationSourcesBeforeBodyLinks(t *testing.T) {
	text := "Answer ([Example](https://example.com/a)).\n\nsources([{\"url\":\"https://example.com/a\",\"title\":\"Citation\"}])"

	answer, sources := SplitAnswerAndSources(text)

	if answer != "Answer ([Example](https://example.com/a))." {
		t.Fatalf("answer = %q", answer)
	}
	if len(sources) != 1 {
		t.Fatalf("sources = %#v", sources)
	}
	if sources[0]["title"] != "Citation" || sources[0]["url"] != "https://example.com/a" {
		t.Fatalf("annotation source should win duplicate merge: %#v", sources[0])
	}
}
