import type { SiteSettings } from "~/shared/api/types";
import { cn } from "~/shared/lib/cn";

const platformLabels: Record<string, string> = {
  github: "GitHub",
  x: "X",
  twitter: "Twitter",
  mastodon: "Mastodon",
  email: "邮箱",
  weibo: "微博",
  zhihu: "知乎",
  bilibili: "哔哩哔哩",
};

export function SiteFooter({ settings, narrow }: { settings: SiteSettings; narrow: boolean }) {
  return (
    <footer className={cn("mx-auto box-content max-w-wide px-4 pt-12 pb-16", narrow && "max-w-measure")}>
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 text-md leading-loose text-muted">
        <span>{settings.footer_text || `© ${settings.author}`}</span>
        <div className="flex flex-wrap items-center gap-x-5 gap-y-1">
          {settings.social_links.map((s) => (
            <a key={s.url} href={s.url} className={footerLink} rel="me noopener">
              {platformLabels[s.platform] ?? s.platform}
            </a>
          ))}
          <a href="/feed.xml" className={footerLink}>
            Atom
          </a>
          <a href="/rss.xml" className={footerLink}>
            RSS
          </a>
        </div>
      </div>
    </footer>
  );
}

const footerLink = "inline-flex min-h-11 items-center gap-1.5";
