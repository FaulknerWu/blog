package feed

import "testing"

func TestAbsolutize(t *testing.T) {
	const base = "https://blog.example.com"
	cases := []struct{ in, want string }{
		{`<p>纯文本</p>`, `<p>纯文本</p>`},
		{`<a href="/posts/x">x</a>`, `<a href="https://blog.example.com/posts/x">x</a>`},
		{`<a href="//cdn.example.com/x">x</a>`, `<a href="//cdn.example.com/x">x</a>`},
		{`<a href="https://e.org/">x</a>`, `<a href="https://e.org/">x</a>`},
		{`<img src="/media/a" srcset="/media/a/480.webp 480w, /media/a/960.webp 960w">`,
			`<img src="https://blog.example.com/media/a" srcset="https://blog.example.com/media/a/480.webp 480w, https://blog.example.com/media/a/960.webp 960w">`},
		{`<span style="color:var(--shiki-light)">a &amp; b</span><math><mi>x</mi></math>`,
			`<span style="color:var(--shiki-light)">a &amp; b</span><math><mi>x</mi></math>`},
	}
	for _, c := range cases {
		if got := absolutize(c.in, base); got != c.want {
			t.Errorf("absolutize(%q)\n got  %q\n want %q", c.in, got, c.want)
		}
	}
}
