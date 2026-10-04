package html

import (
	"html/template"
	"net/http"

	"github.com/magomedcoder/kwiki/internal/adapter/markdown"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

type helpItem struct {
	ID     string
	Title  string
	Lead   string
	Source string
	Result template.HTML
	Note   string
}

type helpData struct {
	shell
	Lead   string
	Limits string
	Items  []helpItem
}

func (r *Renderer) Help(w http.ResponseWriter, actor usecase.Actor, lang string) {
	r.exec(w, r.help, helpData{
		shell:  r.actorShell(r.bundle.T(lang, "help.title"), actor, lang),
		Lead:   r.bundle.T(lang, "help.intro"),
		Limits: r.bundle.T(lang, "help.limits"),
		Items:  r.helpItems(lang),
	}, lang)
}

func (r *Renderer) helpItems(lang string) []helpItem {
	t := func(key string) string {
		return r.bundle.T(lang, key)
	}

	live := func(id, prefix, source string) helpItem {
		return helpItem{
			ID:     id,
			Title:  t(prefix + ".title"),
			Lead:   t(prefix + ".lead"),
			Source: source,
			Result: template.HTML(markdown.HTML([]byte(source), "")),
		}
	}

	note := func(id, prefix, source string) helpItem {
		return helpItem{
			ID:     id,
			Title:  t(prefix + ".title"),
			Lead:   t(prefix + ".lead"),
			Source: source,
			Note:   t(prefix + ".note"),
		}
	}

	return []helpItem{
		live("headings", "help.headings", "# "+t("help.headings.h1")+"\n## "+t("help.headings.h2")+"\n### "+t("help.headings.h3")+"\n#### "+t("help.headings.h4")+"\n##### "+t("help.headings.h5")+"\n###### "+t("help.headings.h6")),
		live("paragraphs", "help.paragraphs", t("help.paragraphs.a")+"\n\n"+t("help.paragraphs.b")),
		live("emphasis", "help.emphasis", t("help.emphasis.sample")),
		live("strike", "help.strike", t("help.strike.sample")),
		live("inline-code", "help.inline_code", t("help.inline_code.sample")),
		live("code-block", "help.code_block", "```\nkwiki -data ./data\n```"),
		live("lists", "help.lists", "- "+t("help.lists.a")+"\n- "+t("help.lists.b")+"\n* "+t("help.lists.c")),
		live("ordered", "help.ordered", "1. "+t("help.ordered.a")+"\n2. "+t("help.ordered.b")+"\n3. "+t("help.ordered.c")),
		live("quote", "help.quote", "> "+t("help.quote.a")+"\n> "+t("help.quote.b")),
		live("rule", "help.rule", t("help.rule.before")+"\n\n---\n\n"+t("help.rule.after")),
		live("links", "help.links", t("help.links.sample")),
		note("images", "help.images", "![alt](photo.png)\n![alt](photo.png =400x300)\n![alt](photo.png =400x)\n![alt](photo.png =x240)"),
		note("video", "help.video", "![clip](clip.mp4)\n![clip](clip.mp4 =640x360)"),
		live("table", "help.table", "| "+t("help.table.col_a")+" | "+t("help.table.col_b")+" |\n| --- | --- |\n| "+t("help.table.cell_a")+" | "+t("help.table.cell_b")+" |"),
		live("combine", "help.combine", t("help.combine.sample")),
	}
}
