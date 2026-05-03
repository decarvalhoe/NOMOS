package fidelity

import (
	"testing"
)

func TestGFM_TaskListItems(t *testing.T) {
	src := loadFixture(t, "gfm-sample.md")
	cast := ParseMarkdown(src)

	if cast.Coverage.TaskListItems < 3 {
		t.Fatalf("expected at least 3 task list items, got %d", cast.Coverage.TaskListItems)
	}

	tasks := findNodes(cast, KindTaskListItem)
	if len(tasks) < 3 {
		t.Fatalf("expected at least 3 task list item nodes, got %d", len(tasks))
	}

	checkedCount := 0
	uncheckedCount := 0
	for _, task := range tasks {
		if task.Props["checked"] == "true" {
			checkedCount++
		} else {
			uncheckedCount++
		}
	}
	if checkedCount < 2 {
		t.Fatalf("expected at least 2 checked tasks, got %d", checkedCount)
	}
	if uncheckedCount < 1 {
		t.Fatalf("expected at least 1 unchecked task, got %d", uncheckedCount)
	}
}

func TestGFM_TaskListType(t *testing.T) {
	src := "# Doc\n\n- [x] Done\n- [ ] Todo\n"
	cast := ParseMarkdown(src)
	lists := findNodes(cast, KindList)
	if len(lists) != 1 {
		t.Fatalf("expected 1 list, got %d", len(lists))
	}
	if lists[0].Props["list_type"] != "task" {
		t.Fatalf("expected list_type=task, got %q", lists[0].Props["list_type"])
	}
}

func TestGFM_Strikethrough(t *testing.T) {
	src := loadFixture(t, "gfm-sample.md")
	cast := ParseMarkdown(src)

	if cast.Coverage.Strikethroughs < 2 {
		t.Fatalf("expected at least 2 strikethroughs, got %d", cast.Coverage.Strikethroughs)
	}

	// Check paragraph props contain strikethrough.
	found := false
	for _, n := range cast.Nodes {
		if n.Props != nil && n.Props["strikethrough_0"] != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected strikethrough_0 prop on a node")
	}
}

func TestGFM_StrikethroughContent(t *testing.T) {
	src := "Text with ~~deleted~~ word.\n"
	cast := ParseMarkdown(src)
	paras := findNodes(cast, KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Props["strikethrough_0"] != "deleted" {
		t.Fatalf("expected strikethrough_0=deleted, got %q", paras[0].Props["strikethrough_0"])
	}
}

func TestGFM_Autolinks(t *testing.T) {
	src := loadFixture(t, "gfm-sample.md")
	cast := ParseMarkdown(src)

	if cast.Coverage.Autolinks < 2 {
		t.Fatalf("expected at least 2 autolinks, got %d", cast.Coverage.Autolinks)
	}
}

func TestGFM_AutolinkAngleBracket(t *testing.T) {
	src := "Visit <https://example.com> here.\n"
	cast := ParseMarkdown(src)
	paras := findNodes(cast, KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Props["autolink_0"] != "https://example.com" {
		t.Fatalf("expected autolink_0, got %v", paras[0].Props)
	}
}

func TestGFM_AutolinkBare(t *testing.T) {
	src := "See https://github.com/test for info.\n"
	cast := ParseMarkdown(src)
	paras := findNodes(cast, KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Props["autolink_bare_0"] != "https://github.com/test" {
		t.Fatalf("expected autolink_bare_0, got %v", paras[0].Props)
	}
}

func TestGFM_FootnoteDefinitions(t *testing.T) {
	src := loadFixture(t, "gfm-sample.md")
	cast := ParseMarkdown(src)

	if cast.Coverage.FootnoteDefs < 2 {
		t.Fatalf("expected at least 2 footnote defs, got %d", cast.Coverage.FootnoteDefs)
	}

	fnDefs := findNodes(cast, KindFootnoteDef)
	if len(fnDefs) < 2 {
		t.Fatalf("expected at least 2 footnote def nodes, got %d", len(fnDefs))
	}
	// Check label prop.
	if fnDefs[0].Props["label"] == "" {
		t.Fatal("expected label prop on footnote def")
	}
}

func TestGFM_FootnoteReferences(t *testing.T) {
	src := loadFixture(t, "gfm-sample.md")
	cast := ParseMarkdown(src)

	if cast.Coverage.FootnoteRefs < 2 {
		t.Fatalf("expected at least 2 footnote refs, got %d", cast.Coverage.FootnoteRefs)
	}
}

func TestGFM_FootnoteRefInProps(t *testing.T) {
	src := "Claim needs proof[^ref1].\n"
	cast := ParseMarkdown(src)
	paras := findNodes(cast, KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Props["footnote_ref_0"] != "ref1" {
		t.Fatalf("expected footnote_ref_0=ref1, got %v", paras[0].Props)
	}
}

func TestGFM_MixedTaskWithStrikethrough(t *testing.T) {
	src := "# Doc\n\n- [x] Done with ~~old~~ text\n"
	cast := ParseMarkdown(src)
	tasks := findNodes(cast, KindTaskListItem)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task item, got %d", len(tasks))
	}
	if tasks[0].Props["checked"] != "true" {
		t.Fatal("expected checked=true")
	}
	if tasks[0].Props["strikethrough_0"] != "old" {
		t.Fatalf("expected strikethrough in task props, got %v", tasks[0].Props)
	}
}

func TestGFM_FullCoverage(t *testing.T) {
	src := loadFixture(t, "gfm-sample.md")
	cast := ParseMarkdown(src)
	cov := cast.Coverage

	gfmChecks := map[string]int{
		"task_list_items": cov.TaskListItems,
		"strikethroughs":  cov.Strikethroughs,
		"autolinks":       cov.Autolinks,
		"footnote_defs":   cov.FootnoteDefs,
		"footnote_refs":   cov.FootnoteRefs,
	}
	for name, count := range gfmChecks {
		if count == 0 {
			t.Fatalf("GFM coverage.%s is 0", name)
		}
	}

	// CommonMark elements should still work.
	cmChecks := map[string]int{
		"headings":   cov.Headings,
		"paragraphs": cov.Paragraphs,
		"tables":     cov.Tables,
	}
	for name, count := range cmChecks {
		if count == 0 {
			t.Fatalf("CommonMark coverage.%s is 0 in GFM doc", name)
		}
	}
}

func TestGFM_CommonMarkStillWorks(t *testing.T) {
	// Ensure original CommonMark fixture still passes.
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	cov := cast.Coverage

	if cov.Headings == 0 || cov.Paragraphs == 0 || cov.Lists == 0 ||
		cov.CodeBlocks == 0 || cov.Blockquotes == 0 || cov.Tables == 0 ||
		cov.ThematicBreaks == 0 || cov.Links == 0 || cov.Images == 0 {
		t.Fatal("CommonMark coverage regressed after GFM additions")
	}
}
