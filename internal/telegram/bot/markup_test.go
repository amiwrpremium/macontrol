package bot

import "testing"

func TestMDToHTML(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "hello world", "hello world"},
		{"bold", "hello *world*", "hello <b>world</b>"},
		{"italic", "hello _world_", "hello <i>world</i>"},
		{"code", "run `echo hi`", "run <code>echo hi</code>"},
		{"fence", "pre ```\ncode\n``` post", "pre <pre>\ncode\n</pre> post"},
		{"mixed", "*bold* and _italic_ and `code`", "<b>bold</b> and <i>italic</i> and <code>code</code>"},
		{"html escape lt", "a < b", "a &lt; b"},
		{"html escape amp", "a & b", "a &amp; b"},
		{"html escape gt", "a > b", "a &gt; b"},
		{"dots preserved", "v0.1.1 is here", "v0.1.1 is here"},
		{"dynamic with dots inside code", "run `v0.1.1` now", "run <code>v0.1.1</code> now"},
		{"orphan backtick", "cost $100`", "cost $100`"},
		{"orphan asterisk", "2 * 3 = 6", "2 * 3 = 6"},
		{"two pairs", "*a* and *b*", "<b>a</b> and <b>b</b>"},
		{"three open = two pairs + orphan", "*a* *b *c", "<b>a</b> <b>b </b>c"},
		{"nested tags via fence first", "```\n_inside_\n```", "<pre>\n<i>inside</i>\n</pre>"},
		{"dynamic bracket content", "<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MDToHTML(tc.in)
			if got != tc.want {
				t.Errorf("MDToHTML(%q)\n  got:  %q\n  want: %q", tc.in, got, tc.want)
			}
		})
	}
}
