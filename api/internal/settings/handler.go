package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"blog/api/internal/httpx"
	"blog/api/internal/store"
)

const maxLinks = 10

var (
	languagePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$`)
	platformPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler { return &Handler{store: s} }

type navItem struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type socialLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type publicSettings struct {
	Title        string       `json:"title"`
	Subtitle     string       `json:"subtitle"`
	Description  string       `json:"description"`
	Author       string       `json:"author"`
	Language     string       `json:"language"`
	Timezone     string       `json:"timezone"`
	FooterText   string       `json:"footer_text"`
	Navigation   []navItem    `json:"navigation"`
	SocialLinks  []socialLink `json:"social_links"`
	Noindex      bool         `json:"noindex"`
	PostsPerPage int32        `json:"posts_per_page"`
}

type adminSettings struct {
	publicSettings
	FeedItemCount int32      `json:"feed_item_count"`
	Version       int32      `json:"version"`
	UpdatedAt     httpx.Time `json:"updated_at"`
}

func decodeList[T any](raw json.RawMessage) ([]T, error) {
	out := []T{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func toPublic(s store.SiteSetting) (publicSettings, error) {
	nav, err := decodeList[navItem](s.Navigation)
	if err != nil {
		return publicSettings{}, fmt.Errorf("decode navigation: %w", err)
	}
	social, err := decodeList[socialLink](s.SocialLinks)
	if err != nil {
		return publicSettings{}, fmt.Errorf("decode social_links: %w", err)
	}
	return publicSettings{
		Title: s.Title, Subtitle: s.Subtitle, Description: s.Description, Author: s.Author,
		Language: s.Language, Timezone: s.Timezone, FooterText: s.FooterText,
		Navigation: nav, SocialLinks: social, Noindex: s.Noindex, PostsPerPage: s.PostsPerPage,
	}, nil
}

func toAdmin(s store.SiteSetting) (adminSettings, error) {
	p, err := toPublic(s)
	if err != nil {
		return adminSettings{}, err
	}
	return adminSettings{
		publicSettings: p,
		FeedItemCount:  s.FeedItemCount,
		Version:        s.Version,
		UpdatedAt:      httpx.NewTime(s.UpdatedAt),
	}, nil
}

type settingsInput struct {
	Title         *string       `json:"title"`
	Subtitle      *string       `json:"subtitle"`
	Description   *string       `json:"description"`
	Author        *string       `json:"author"`
	Language      *string       `json:"language"`
	Timezone      *string       `json:"timezone"`
	FooterText    *string       `json:"footer_text"`
	Navigation    *[]navItem    `json:"navigation"`
	SocialLinks   *[]socialLink `json:"social_links"`
	Noindex       *bool         `json:"noindex"`
	PostsPerPage  *int32        `json:"posts_per_page"`
	FeedItemCount *int32        `json:"feed_item_count"`
}

func (in settingsInput) empty() bool {
	return in.Title == nil && in.Subtitle == nil && in.Description == nil && in.Author == nil &&
		in.Language == nil && in.Timezone == nil && in.FooterText == nil && in.Navigation == nil &&
		in.SocialLinks == nil && in.Noindex == nil && in.PostsPerPage == nil && in.FeedItemCount == nil
}

func validTimezone(name string) bool {
	// LoadLocation 对 "" 和 "Local" 也返回成功，但它们不是 IANA 时区名。
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

func navURL(raw string) bool {
	if strings.HasPrefix(raw, "/") {
		// 浏览器把 // 和 /\ 开头的地址当作协议相对 URL，会跳到外站。
		return !strings.HasPrefix(raw, "//") && !strings.HasPrefix(raw, `/\`)
	}
	return absoluteURL(raw)
}

func absoluteURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func socialURL(raw string) bool {
	if addr, ok := strings.CutPrefix(raw, "mailto:"); ok {
		return strings.Contains(addr, "@")
	}
	return absoluteURL(raw)
}

func validateLinkURL(v *httpx.Validator, pointer, raw string, valid func(string) bool, detail string) {
	if v.Length(pointer, raw, 1, 500) && !valid(raw) {
		v.Add(pointer, "invalid_format", detail)
	}
}

type siteSettings struct {
	store.UpdateSiteSettingsParams
	nav    []navItem
	social []socialLink
}

func validate(s siteSettings) error {
	var v httpx.Validator
	v.Length("#/title", s.Title, 1, 100)
	v.Length("#/subtitle", s.Subtitle, 0, 200)
	v.Length("#/description", s.Description, 0, 500)
	v.Length("#/author", s.Author, 1, 100)
	if v.Length("#/language", s.Language, 1, 35) && !languagePattern.MatchString(s.Language) {
		v.Add("#/language", "invalid_format", "须为 BCP 47 语言标签，如 zh-CN、en")
	}
	if !validTimezone(s.Timezone) {
		v.Add("#/timezone", "invalid_value", "须为 IANA 时区名称，如 Asia/Shanghai")
	}
	v.Length("#/footer_text", s.FooterText, 0, 500)
	if len(s.nav) > maxLinks {
		v.Add("#/navigation", "too_many", fmt.Sprintf("最多 %d 项", maxLinks))
	}
	for i, item := range s.nav {
		p := fmt.Sprintf("#/navigation/%d", i)
		v.Length(p+"/label", item.Label, 1, 50)
		validateLinkURL(&v, p+"/url", item.URL, navURL, "须为以 / 开头的站内路径或 http(s) 地址")
	}
	if len(s.social) > maxLinks {
		v.Add("#/social_links", "too_many", fmt.Sprintf("最多 %d 项", maxLinks))
	}
	for i, link := range s.social {
		p := fmt.Sprintf("#/social_links/%d", i)
		if v.Length(p+"/platform", link.Platform, 1, 30) && !platformPattern.MatchString(link.Platform) {
			v.Add(p+"/platform", "invalid_format", "只能包含小写字母、数字和连字符，如 github、x、email")
		}
		validateLinkURL(&v, p+"/url", link.URL, socialURL, "须为 http(s) 地址或 mailto: 邮箱地址")
	}
	v.Range("#/posts_per_page", s.PostsPerPage, 1, 50)
	v.Range("#/feed_item_count", s.FeedItemCount, 1, 100)
	return v.Err()
}

func setTrimmed(dst *string, src *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}

func (h *Handler) Public(w http.ResponseWriter, r *http.Request) error {
	s, err := h.store.GetSiteSettings(r.Context())
	if err != nil {
		return err
	}
	p, err := toPublic(s)
	if err != nil {
		return err
	}
	return httpx.WritePublicJSON(w, r, p)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	s, err := h.store.GetSiteSettings(r.Context())
	if err != nil {
		return err
	}
	a, err := toAdmin(s)
	if err != nil {
		return err
	}
	httpx.SetETag(w, s.Version)
	return httpx.WriteJSON(w, http.StatusOK, a)
}

// Update 是 PATCH 语义：只覆盖请求中出现的字段，与现有配置合并后整体校验。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) error {
	var in settingsInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	err := h.store.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockSiteSettings(ctx)
		if err != nil {
			return err
		}
		if err := httpx.ParseIfMatch(r).Check(cur.Version, cur.UpdatedAt); err != nil {
			return err
		}
		if in.empty() {
			return nil
		}
		old, err := toPublic(cur)
		if err != nil {
			return err
		}
		s := siteSettings{
			UpdateSiteSettingsParams: store.UpdateSiteSettingsParams{
				Title: cur.Title, Subtitle: cur.Subtitle, Description: cur.Description, Author: cur.Author,
				Language: cur.Language, Timezone: cur.Timezone, FooterText: cur.FooterText,
				Noindex: cur.Noindex, PostsPerPage: cur.PostsPerPage, FeedItemCount: cur.FeedItemCount,
			},
			nav:    old.Navigation,
			social: old.SocialLinks,
		}
		setTrimmed(&s.Title, in.Title)
		setTrimmed(&s.Subtitle, in.Subtitle)
		setTrimmed(&s.Description, in.Description)
		setTrimmed(&s.Author, in.Author)
		setTrimmed(&s.Language, in.Language)
		setTrimmed(&s.Timezone, in.Timezone)
		setTrimmed(&s.FooterText, in.FooterText)
		if in.Navigation != nil {
			s.nav = make([]navItem, 0, len(*in.Navigation))
			for _, item := range *in.Navigation {
				s.nav = append(s.nav, navItem{Label: strings.TrimSpace(item.Label), URL: strings.TrimSpace(item.URL)})
			}
		}
		if in.SocialLinks != nil {
			s.social = make([]socialLink, 0, len(*in.SocialLinks))
			for _, link := range *in.SocialLinks {
				s.social = append(s.social, socialLink{Platform: strings.TrimSpace(link.Platform), URL: strings.TrimSpace(link.URL)})
			}
		}
		if in.Noindex != nil {
			s.Noindex = *in.Noindex
		}
		if in.PostsPerPage != nil {
			s.PostsPerPage = *in.PostsPerPage
		}
		if in.FeedItemCount != nil {
			s.FeedItemCount = *in.FeedItemCount
		}
		if err := validate(s); err != nil {
			return err
		}
		if s.Navigation, err = json.Marshal(s.nav); err != nil {
			return err
		}
		if s.SocialLinks, err = json.Marshal(s.social); err != nil {
			return err
		}
		return q.UpdateSiteSettings(ctx, s.UpdateSiteSettingsParams)
	})
	if err != nil {
		return err
	}
	return h.Get(w, r)
}
