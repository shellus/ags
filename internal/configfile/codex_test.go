package configfile

import (
	"strings"
	"testing"
)

func TestUpdateCodexDisabledSkillsPreservesConfigAndIsIdempotent(t *testing.T) {
	input := "model = \"demo\"\n\n[[skills.config]]\nname = \"manual\"\nenabled = true\n"
	updated, err := UpdateCodexDisabledSkills(input, []string{"openai-docs", "imagegen"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		input,
		codexDisabledSkillsBegin,
		"name = \"imagegen\"",
		"name = \"openai-docs\"",
		codexDisabledSkillsEnd,
	} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("updated config does not contain %q:\n%s", expected, updated)
		}
	}
	second, err := UpdateCodexDisabledSkills(updated, []string{"openai-docs", "imagegen"})
	if err != nil {
		t.Fatal(err)
	}
	if second != updated {
		t.Fatalf("second update changed config:\n%s", second)
	}
	removed, err := UpdateCodexDisabledSkills(updated, nil)
	if err != nil {
		t.Fatal(err)
	}
	if removed != input {
		t.Fatalf("removed block did not restore input:\n%s", removed)
	}
}

func TestUpdateCodexDisabledSkillsUsesExistingNewlines(t *testing.T) {
	updated, err := UpdateCodexDisabledSkills("model = \"demo\"\r\n", []string{"openai-docs"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(updated, "\r\n", ""), "\n") {
		t.Fatalf("mixed newline styles:\n%q", updated)
	}
}

func TestUpdateCodexDisabledSkillsRejectsBrokenMarkers(t *testing.T) {
	for _, input := range []string{
		codexDisabledSkillsBegin + "\n",
		codexDisabledSkillsEnd + "\n",
		codexDisabledSkillsBegin + "\n" + codexDisabledSkillsBegin + "\n" + codexDisabledSkillsEnd + "\n",
	} {
		if _, err := UpdateCodexDisabledSkills(input, []string{"openai-docs"}); err == nil {
			t.Fatalf("broken marker config was accepted: %q", input)
		}
	}
}
