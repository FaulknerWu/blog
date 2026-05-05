package feed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

type Config struct {
	SiteURL string
}

func (c Config) postURL(slug string) string   { return c.SiteURL + "/posts/" + url.PathEscape(slug) }
func (c Config) tagURL(slug string) string    { return c.SiteURL + "/tags/" + url.PathEscape(slug) }
func (c Config) seriesURL(slug string) string { return c.SiteURL + "/series/" + url.PathEscape(slug) }
func (c Config) pageURL(slug string) string   { return c.SiteURL + "/" + url.PathEscape(slug) }

// entryID 生成 RFC 4151 tag URI 作为条目的永久 ID，不含 slug，改 slug 后阅读器不会把它当作新文章。
func (c Config) entryID(id int64, published time.Time) string {
	host := c.SiteURL
	if u, err := url.Parse(c.SiteURL); err == nil {
		host = u.Host
	}
	return fmt.Sprintf("tag:%s,%s:post:%d", host, published.UTC().Format("2006-01-02"), id)
}

type Handler struct {
	store *store.Store
	cfg   Config
}

func NewHandler(s *store.Store, cfg Config) *Handler { return &Handler{store: s, cfg: cfg} }

func lastModified(rows []store.ListFeedPostsRow) time.Time {
	var t time.Time
	for _, r := range rows {
		t = latest(t, r.UpdatedAt, r.PublishedAt)
	}
	return t
}

func latest(ts ...time.Time) time.Time {
	var out time.Time
	for _, t := range ts {
		if t.After(out) {
			out = t
		}
	}
	return out
}

func serveXML(w http.ResponseWriter, r *http.Request, contentType string, modified time.Time, v any) error {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	body := buf.Bytes()
	sum := sha256.Sum256(body)
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "public, max-age=300")
	h.Set("ETag", `"`+hex.EncodeToString(sum[:12])+`"`)
	http.ServeContent(w, r, "", modified.Truncate(time.Second), bytes.NewReader(body))
	return nil
}

type atomLink struct {
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomText struct {
	Type string `xml:"type,attr,omitempty"`
	Body string `xml:",chardata"`
}

type atomEntry struct {
	ID        string    `xml:"id"`
	Title     string    `xml:"title"`
	Link      atomLink  `xml:"link"`
	Published string    `xml:"published"`
	Updated   string    `xml:"updated"`
	Summary   *atomText `xml:"summary,omitempty"`
	Content   atomText  `xml:"content"`
}

type atomFeed struct {
	XMLName  xml.Name   `xml:"http://www.w3.org/2005/Atom feed"`
	Lang     string     `xml:"xml:lang,attr,omitempty"`
	ID       string     `xml:"id"`
	Title    string     `xml:"title"`
	Subtitle string     `xml:"subtitle,omitempty"`
	Updated  string     `xml:"updated"`
	Links    []atomLink `xml:"link"`
	Author   struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Entries []atomEntry `xml:"entry"`
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (h *Handler) loadFeed(r *http.Request) (store.SiteSetting, []store.ListFeedPostsRow, time.Time, error) {
	ctx := r.Context()
	st, err := h.store.GetSiteSettings(ctx)
	if err != nil {
		return st, nil, time.Time{}, err
	}
	rows, err := h.store.ListFeedPosts(ctx, st.FeedItemCount)
	if err != nil {
		return st, nil, time.Time{}, err
	}
	return st, rows, latest(lastModified(rows), st.UpdatedAt), nil
}

func setRobots(w http.ResponseWriter, st store.SiteSetting) {
	if st.Noindex {
		w.Header().Set("X-Robots-Tag", "noindex")
	}
}

func (h *Handler) Atom(w http.ResponseWriter, r *http.Request) error {
	st, rows, mod, err := h.loadFeed(r)
	if err != nil {
		return err
	}
	setRobots(w, st)
	f := atomFeed{
		Lang:     st.Language,
		ID:       h.cfg.SiteURL + "/",
		Title:    st.Title,
		Subtitle: st.Description,
		Updated:  rfc3339(latest(mod, time.Unix(0, 0))),
		Links: []atomLink{
			{Rel: "alternate", Type: "text/html", Href: h.cfg.SiteURL + "/"},
			{Rel: "self", Type: "application/atom+xml", Href: h.cfg.SiteURL + "/feed.xml"},
		},
	}
	f.Author.Name = st.Author
	for _, p := range rows {
		e := atomEntry{
			ID:        h.cfg.entryID(p.ID, p.PublishedAt),
			Title:     p.Title,
			Link:      atomLink{Rel: "alternate", Type: "text/html", Href: h.cfg.postURL(p.Slug)},
			Published: rfc3339(p.PublishedAt),
			Updated:   rfc3339(latest(p.UpdatedAt, p.PublishedAt)),
			Content:   atomText{Type: "html", Body: absolutize(p.BodyHtml, h.cfg.SiteURL)},
		}
		if p.Summary != "" {
			e.Summary = &atomText{Type: "text", Body: p.Summary}
		}
		f.Entries = append(f.Entries, e)
	}
	return serveXML(w, r, "application/atom+xml; charset=utf-8", mod, f)
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
	Description string  `xml:"description"`
}

type rssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	AtomNS  string   `xml:"xmlns:atom,attr"`
	Channel struct {
		Title         string    `xml:"title"`
		Link          string    `xml:"link"`
		Description   string    `xml:"description"`
		Language      string    `xml:"language,omitempty"`
		LastBuildDate string    `xml:"lastBuildDate,omitempty"`
		AtomLink      atomLink  `xml:"atom:link"`
		Items         []rssItem `xml:"item"`
	} `xml:"channel"`
}

func (h *Handler) RSS(w http.ResponseWriter, r *http.Request) error {
	st, rows, mod, err := h.loadFeed(r)
	if err != nil {
		return err
	}
	setRobots(w, st)
	var f rssFeed
	f.Version = "2.0"
	f.AtomNS = "http://www.w3.org/2005/Atom"
	c := &f.Channel
	c.Title = st.Title
	c.Link = h.cfg.SiteURL + "/"
	c.Description = st.Description
	if c.Description == "" {
		c.Description = st.Title
	}
	c.Language = st.Language
	if !mod.IsZero() {
		c.LastBuildDate = mod.UTC().Format(time.RFC1123Z)
	}
	c.AtomLink = atomLink{Rel: "self", Type: "application/rss+xml", Href: h.cfg.SiteURL + "/rss.xml"}
	for _, p := range rows {
		c.Items = append(c.Items, rssItem{
			Title:       p.Title,
			Link:        h.cfg.postURL(p.Slug),
			GUID:        rssGUID{IsPermaLink: "false", Value: h.cfg.entryID(p.ID, p.PublishedAt)},
			PubDate:     p.PublishedAt.UTC().Format(time.RFC1123Z),
			Description: absolutize(p.BodyHtml, h.cfg.SiteURL),
		})
	}
	return serveXML(w, r, "application/rss+xml; charset=utf-8", mod, f)
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type urlset struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []sitemapURL `xml:"url"`
}

func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	st, err := h.store.GetSiteSettings(ctx)
	if err != nil {
		return err
	}
	if st.Noindex {
		return httpx.NotFound("站点已禁止搜索引擎收录")
	}
	posts, err := h.store.ListSitemapPosts(ctx)
	if err != nil {
		return err
	}
	pages, err := h.store.ListSitemapPages(ctx)
	if err != nil {
		return err
	}
	tags, err := h.store.ListSitemapTags(ctx)
	if err != nil {
		return err
	}
	series, err := h.store.ListSitemapSeries(ctx)
	if err != nil {
		return err
	}
	var mod time.Time
	for _, p := range posts {
		mod = latest(mod, p.UpdatedAt)
	}
	for _, p := range pages {
		mod = latest(mod, p.UpdatedAt)
	}
	for _, t := range tags {
		mod = latest(mod, t.LastModified)
	}
	for _, s := range series {
		mod = latest(mod, s.LastModified)
	}

	set := urlset{}
	home := sitemapURL{Loc: h.cfg.SiteURL + "/"}
	if !mod.IsZero() {
		home.LastMod = rfc3339(mod)
	}
	set.URLs = append(set.URLs, home)
	for _, p := range posts {
		set.URLs = append(set.URLs, sitemapURL{Loc: h.cfg.postURL(p.Slug), LastMod: rfc3339(p.UpdatedAt)})
	}
	for _, p := range pages {
		set.URLs = append(set.URLs, sitemapURL{Loc: h.cfg.pageURL(p.Slug), LastMod: rfc3339(p.UpdatedAt)})
	}
	for _, t := range tags {
		set.URLs = append(set.URLs, sitemapURL{Loc: h.cfg.tagURL(t.Slug), LastMod: rfc3339(t.LastModified)})
	}
	for _, s := range series {
		set.URLs = append(set.URLs, sitemapURL{Loc: h.cfg.seriesURL(s.Slug), LastMod: rfc3339(s.LastModified)})
	}
	return serveXML(w, r, "application/xml; charset=utf-8", mod, set)
}

func (h *Handler) Robots(w http.ResponseWriter, r *http.Request) error {
	st, err := h.store.GetSiteSettings(r.Context())
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n")
	if !st.Noindex {
		b.WriteString("\nSitemap: " + h.cfg.SiteURL + "/sitemap.xml\n")
	}
	hd := w.Header()
	hd.Set("Content-Type", "text/plain; charset=utf-8")
	hd.Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, "", st.UpdatedAt.Truncate(time.Second), strings.NewReader(b.String()))
	return nil
}
