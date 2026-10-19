// 接口数据结构的命名别名，全部来自 api/openapi.yaml 生成的 schema.gen.ts（npm run gen:api）。
import type { components } from "./schema.gen";

type Schemas = components["schemas"];

export type Timestamp = Schemas["Timestamp"];
export type FieldError = Schemas["FieldError"];
// Problem 合并了各类错误响应的扩展字段：412 的 current_etag/updated_at，删除仍被引用图片时的 referenced_by。
export type Problem = Schemas["Problem"] &
  Partial<Omit<Schemas["PreconditionFailedProblem"], keyof Schemas["Problem"]>> &
  Partial<Omit<Schemas["MediaInUseProblem"], keyof Schemas["Problem"]>>;

// 公开内容
export type TagRef = Schemas["TagRef"];
export type PostRef = Schemas["PostRef"];
export type SeriesRef = Schemas["SeriesRef"];
export type Variant = Schemas["Variant"];
export type Image = Schemas["Image"];
export type TocItem = Schemas["TocItem"];
export type PostSummary = Schemas["PostSummary"];
export type PostSummaryList = Schemas["PostSummaryList"];
export type SeriesNav = Schemas["SeriesNav"];
export type Post = Schemas["Post"];
export type Page = Schemas["Page"];
export type Tag = Schemas["Tag"];
export type TagList = Schemas["TagList"];
export type Series = Schemas["Series"];
export type SeriesList = Schemas["SeriesList"];
export type SeriesDetail = Schemas["SeriesDetail"];
export type SearchResult = Schemas["SearchResult"];
export type SearchResults = Schemas["SearchResults"];
export type SiteSettings = Schemas["SiteSettings"];

// 认证与令牌
export type User = Schemas["User"];
export type LoginResult = Schemas["LoginResult"];
export type Me = Schemas["Me"];
export type Session = Schemas["Session"];
export type SessionList = Schemas["SessionList"];
export type Scope = Schemas["Scope"];
export type Token = Schemas["Token"];
export type TokenList = Schemas["TokenList"];
export type TokenInput = Schemas["TokenInput"];
export type TokenCreated = Schemas["TokenCreated"];

// 渲染
export type RenderIssue = Schemas["RenderIssue"];
export type RenderPreview = Schemas["RenderPreview"];
export type RendererInfo = Schemas["RendererInfo"];

// 后台内容
export type PostState = Schemas["PostState"];
export type PageState = Schemas["PageState"];
export type AdminTagRef = Schemas["AdminTagRef"];
export type AdminPostSummary = Schemas["AdminPostSummary"];
export type AdminPostSummaryList = Schemas["AdminPostSummaryList"];
export type AdminPost = Schemas["AdminPost"];
export type PostInput = Schemas["PostInput"];
export type AdminPageSummary = Schemas["AdminPageSummary"];
export type AdminPageSummaryList = Schemas["AdminPageSummaryList"];
export type AdminPage = Schemas["AdminPage"];
export type PageInput = Schemas["PageInput"];
export type AdminTag = Schemas["AdminTag"];
export type AdminTagList = Schemas["AdminTagList"];
export type TagPatch = Schemas["TagPatch"];
export type AdminSeries = Schemas["AdminSeries"];
export type AdminSeriesList = Schemas["AdminSeriesList"];
export type AdminSeriesDetail = Schemas["AdminSeriesDetail"];
export type SeriesInput = Schemas["SeriesInput"];
export type SeriesPatch = Schemas["SeriesPatch"];
export type Media = Schemas["Media"];
export type MediaList = Schemas["MediaList"];
export type MediaReference = Schemas["MediaReference"];
export type MediaDetail = Schemas["MediaDetail"];
export type AdminSettings = Schemas["AdminSettings"];
export type SettingsInput = Schemas["SettingsInput"];
