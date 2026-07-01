package service

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/providers"
)

var deepAllowedTools = []string{"context7 query-docs", "context7 resolve-library-id", "crawl", "exa similar", "exa web-search", "fetch", "map", "repo-wiki", "search"}

func (s *Service) DeepPlan(query, budget, evidenceDir string) map[string]any {
	start := time.Now()
	question := strings.TrimSpace(query)
	if budget == "" {
		budget = "standard"
	}
	if budget != "quick" && budget != "standard" && budget != "deep" {
		budget = "standard"
	}
	evidenceRoot := strings.TrimSpace(evidenceDir)
	if evidenceRoot == "" {
		evidenceRoot = filepath.Join("C:/tmp/onesearch-evidence", time.Now().Format("20060102-1504")+"-"+slugify(question))
	}
	urls := extractURLs(question)
	knownURL := len(urls) > 0
	docsIntent := isDocsIntent(question)
	zhCurrentIntent := isZHCurrentIntent(question)
	recency := "none"
	if containsAny(question, []string{"今天", "实时", "刚刚", "当前", "现在", "today", "current", "live", "realtime"}) || zhCurrentIntent {
		recency = "current"
	} else if containsAny(question, []string{"行情", "价格", "走势", "币圈", "股票", "市场"}) && containsAny(question, []string{"最近", "最新", "recent", "latest"}) {
		recency = "current"
	} else if containsAny(question, []string{"最近", "最新", "recent", "latest"}) {
		recency = "recent"
	}
	scope := "global"
	if knownURL {
		scope = "known_domains"
	} else if containsAny(question, []string{"中国", "国内", "中文", "政策", "监管", "公告", "A股", "港股"}) {
		scope = "china"
	}
	claimRisk := "medium"
	if recency != "none" || containsAny(question, []string{"核验", "验证", "真假", "价格", "行情", "财经", "医疗", "政策", "监管", "risk"}) {
		claimRisk = "high"
	}
	crossValidation := "normal"
	if claimRisk == "high" || containsAny(question, []string{"对比", "选型", "核验", "验证", "compare", "versus"}) {
		crossValidation = "high"
	}
	authority := "normal"
	if docsIntent || claimRisk == "high" || containsAny(question, []string{"官方", "文档", "论文", "标准", "政策", "监管", "official"}) {
		authority = "high"
	}
	complex := budget == "deep" || containsAny(question, []string{"对比", "选型", "核验", "验证", "为什么", "架构", "方案", "趋势", "优缺点", "风险", "区别", "怎么选", "compare", "comparison", "evaluate", "architecture", "tradeoff", "risk"})
	difficulty := "standard"
	if complex {
		difficulty = "high"
	}
	intentSignals := map[string]any{
		"recency_requirement":   recency,
		"docs_api_intent":       docsIntent,
		"locale_domain_scope":   scope,
		"known_url":             knownURL,
		"source_authority_need": authority,
		"claim_risk":            claimRisk,
		"cross_validation_need": crossValidation,
		"breadth_depth_budget":  budget,
	}
	var decomposition []map[string]any
	var capabilities []map[string]any
	var steps []map[string]any
	addStep := func(subID, tool, purpose, command, output string) {
		steps = append(steps, map[string]any{"id": "s" + stringValue(len(steps)+1), "subquestion_id": subID, "tool": tool, "purpose": purpose, "command": command, "output_path": filepath.Join(evidenceRoot, output)})
	}
	nextFile := func(suffix string) string {
		return fmtNumber(len(steps)+1) + "-" + suffix
	}
	searchCommand := func(q string, extra int, filename string) string {
		return "onesearch search " + quoteArg(q) + " --validation balanced --extra-sources " + stringValue(extra) + " --format json --output " + quoteArg(filepath.Join(evidenceRoot, filename))
	}
	exaCommand := func(q string, filename string) string {
		return "onesearch exa web-search " + quoteArg(q) + " --num-results 5 --format json --output " + quoteArg(filepath.Join(evidenceRoot, filename))
	}
	currentSourceCommand := func(q string, filename string) string {
		return "onesearch search " + quoteArg(q) + " --validation strict --extra-sources 3 --format json --output " + quoteArg(filepath.Join(evidenceRoot, filename))
	}
	fetchCommand := func(target, filename string) string {
		return "onesearch fetch " + quoteArg(target) + " --format markdown --output " + quoteArg(filepath.Join(evidenceRoot, filename))
	}

	if knownURL {
		url := urls[0]
		decomposition = append(decomposition,
			deepSubquestion("sq1", "这个已知来源页面本身说了什么？"+url, "用户已经给出 URL，Deep Research 必须先抓正文再扩展。", []string{"page_evidence"}),
			deepSubquestion("sq2", "围绕这个已知来源还需要哪些相邻来源或交叉来源？", "已知好 URL 适合用相似页面和广泛发现扩展证据。", []string{"adjacent_source_discovery", "broad_discovery"}),
		)
		capabilities = append(capabilities,
			deepCapability("page_evidence", []string{"fetch"}, "Fetch the user-provided URL before making claims."),
			deepCapability("adjacent_source_discovery", []string{"exa similar"}, "Find pages adjacent to the known source."),
			deepCapability("broad_discovery", []string{"search"}, "Broaden the context if the fetched page leaves gaps."),
		)
		addStep("sq1", "fetch", "fetch user supplied URL first", fetchCommand(url, "01-fetch.md"), "01-fetch.md")
		addStep("sq2", "exa similar", "find adjacent sources from the provided URL", "onesearch exa similar "+quoteArg(url)+" --num-results 5 --format json --output "+quoteArg(filepath.Join(evidenceRoot, "02-similar.json")), "02-similar.json")
		addStep("sq2", "search", "broad discovery for missing context", searchCommand(question, 1, "03-search.json"), "03-search.json")
	} else {
		decomposition = append(decomposition, deepSubquestion("sq1", question+" 的整体问题轮廓和候选来源是什么？", "先做 broad discovery，避免一开始把问题拆错。", []string{"broad_discovery"}))
		capabilities = append(capabilities, deepCapability("broad_discovery", []string{"search"}, "Find the initial answer shape and candidate sources."))
		extra := 3
		if budget == "quick" {
			extra = 1
		}
		addStep("sq1", "search", "broad discovery and routing metadata", searchCommand(question, extra, "01-search.json"), "01-search.json")
		if docsIntent {
			decomposition = append(decomposition, deepSubquestion("sq2", question+" 的官方文档、API 或 SDK 证据在哪里？", "docs/API intent requires low-noise documentation discovery.", []string{"docs_source_discovery", "page_evidence"}))
			capabilities = append(capabilities, deepCapability("docs_source_discovery", []string{"exa web-search", "context7 resolve-library-id", "context7 query-docs"}, "Find official/API/library documentation before summarizing implementation guidance."))
			file := nextFile("exa.json")
			addStep("sq2", "exa web-search", "official docs and API source discovery", exaCommand(question, file), file)
			libFile := nextFile("context7-library.json")
			addStep("sq2", "context7 resolve-library-id", "resolve library id for docs/API intent", "onesearch context7 resolve-library-id "+quoteArg(libraryHint(question))+" "+quoteArg(question)+" --format json --output "+quoteArg(filepath.Join(evidenceRoot, libFile)), libFile)
			docsFile := nextFile("context7-docs.json")
			addStep("sq2", "context7 query-docs", "retrieve docs after selecting the best library_id", "onesearch context7 query-docs "+quoteArg("<library_id>")+" "+quoteArg(question)+" --format json --output "+quoteArg(filepath.Join(evidenceRoot, docsFile)), docsFile)
		}
		if recency != "none" || scope == "china" {
			subID := "sq" + stringValue(len(decomposition)+1)
			decomposition = append(decomposition, deepSubquestion(subID, question+" 的最新或中文/国内来源如何交叉验证？", "Current or China-scoped prompts need live source discovery without assuming a disabled provider is usable.", []string{"current_or_locale_source_discovery"}))
			capabilities = append(capabilities, deepCapability("current_or_locale_source_discovery", []string{"search"}, "Reinforce Chinese, domestic, or current web evidence through the runtime source_search route."))
			file := nextFile("current-sources.json")
			addStep(subID, "search", "current or locale-specific source discovery through available source_search providers", currentSourceCommand(question, file), file)
		}
		if complex {
			for len(decomposition) < map[bool]int{true: 4, false: 2}[budget == "deep"] {
				subID := "sq" + stringValue(len(decomposition)+1)
				decomposition = append(decomposition, deepSubquestion(subID, question+" 的成本、风险、限制和适用边界是什么？", "High-difficulty research needs downside and boundary checks.", []string{"cross_validation", "page_evidence"}))
			}
			capabilities = append(capabilities, deepCapability("cross_validation", []string{"search", "exa web-search", "fetch"}, "Compare claims across independent sources."))
		}
		if claimRisk == "high" || crossValidation == "high" || len(steps) > 0 {
			file := nextFile("fetch.md")
			subID := "sq1"
			if len(decomposition) > 0 {
				subID = stringValue(decomposition[len(decomposition)-1]["id"])
			}
			addStep(subID, "fetch", "fetch key URLs before final claims", fetchCommand("<key-url>", file), file)
			capabilities = append(capabilities, deepCapability("page_evidence", []string{"fetch"}, "Fetch selected URLs before claim-level synthesis."))
		}
	}

	sort.Strings(deepAllowedTools)
	return map[string]any{
		"ok":                  true,
		"mode":                "deep_research",
		"query_mode":          "deep",
		"question":            question,
		"trigger_source":      "explicit_cli",
		"difficulty":          difficulty,
		"intent_signals":      intentSignals,
		"decomposition":       decomposition,
		"capability_plan":     dedupeCapabilities(capabilities),
		"evidence_policy":     "fetch_before_claim",
		"preflight":           []map[string]any{{"tool": "doctor", "command": "onesearch doctor --format json", "when": "overall configuration readiness is uncertain", "executed_by_deep_command": false}, {"tool": "status", "command": "onesearch status --format json", "when": "choosing a specific capability, provider filter, or provider-direct endpoint", "executed_by_deep_command": false}},
		"steps":               steps,
		"gap_check":           map[string]any{"required": true, "rule": "fetch missing evidence for key claims or downgrade unsupported claims to unverified candidates", "unsupported_claim_action": "downgrade_to_unverified_candidate"},
		"final_answer_policy": "cite fetched evidence, list unverified candidates, and include key commands",
		"usage_boundary":      map[string]any{"search": "onesearch search runs live fast/broad search immediately.", "deep": "onesearch deep is an offline planner; it does not execute provider calls or fetch pages.", "execution": "An AI agent or user executes the listed steps with existing CLI commands, then performs gap_check."},
		"allowed_tools":       deepAllowedTools,
		"evidence_dir":        evidenceRoot,
		"elapsed_ms":          providers.Elapsed(start),
	}
}

func deepSubquestion(id, question, reason string, caps []string) map[string]any {
	return map[string]any{"id": id, "question": question, "reason": reason, "required_capabilities": caps}
}

func deepCapability(capability string, tools []string, reason string) map[string]any {
	return map[string]any{"capability": capability, "tools": tools, "reason": reason}
}

func dedupeCapabilities(items []map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, item := range items {
		key := stringValue(item["capability"])
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func extractURLs(text string) []string {
	re := regexp.MustCompile(`https?://[^\s<>\]\)"']+`)
	var out []string
	for _, match := range re.FindAllString(text, -1) {
		out = append(out, strings.TrimRight(match, ".,;，。；)"))
	}
	return out
}

func containsAny(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func slugify(text string) string {
	text = strings.ToLower(regexp.MustCompile(`https?://`).ReplaceAllString(text, ""))
	slug := regexp.MustCompile(`[^a-z0-9\p{Han}]+`).ReplaceAllString(text, "-")
	slug = strings.Trim(slug, "-")
	if len([]rune(slug)) > 48 {
		return string([]rune(slug)[:48])
	}
	if slug == "" {
		return "deep-research"
	}
	return slug
}

func quoteArg(value string) string {
	value = strings.ReplaceAll(value, "`", "``")
	value = strings.ReplaceAll(value, "$", "`$")
	value = strings.ReplaceAll(value, `"`, "`\"")
	return `"` + value + `"`
}

func libraryHint(question string) string {
	re := regexp.MustCompile(`[A-Za-z][A-Za-z0-9_.-]*`)
	matches := re.FindAllString(question, 2)
	if len(matches) == 0 {
		return "<library-name>"
	}
	return strings.Join(matches, " ")
}

func fmtNumber(value int) string {
	if value < 10 {
		return "0" + stringValue(value)
	}
	return stringValue(value)
}
