import { Link, useLocation } from "react-router";
import type { SiteSettings } from "~/shared/api/types";
import { cn } from "~/shared/lib/cn";

interface NavItem {
  label: string;
  url: string;
  // 文章详情页也算在「文章」栏目下。
  match?: (pathname: string) => boolean;
}

const builtinNav: NavItem[] = [
  { label: "文章", url: "/", match: (p) => p === "/" || p.startsWith("/posts/") },
  { label: "专栏", url: "/series" },
  { label: "标签", url: "/tags" },
];

const searchNav: NavItem = { label: "搜索", url: "/search" };

export function SiteHeader({ settings, masthead }: { settings: SiteSettings; masthead: boolean }) {
  const items = [...builtinNav, ...settings.navigation, searchNav];

  if (masthead) {
    return (
      <header>
        <div className="mx-auto box-content flex max-w-measure flex-col items-center gap-3 px-4 pt-18 text-center">
          <h1 className="text-6xl leading-tight font-bold tracking-wide">
            <Link to="/">{settings.title}</Link>
          </h1>
          {settings.subtitle && <p className="text-base leading-loose text-muted italic">{settings.subtitle}</p>}
          <Nav items={items} className="mt-5 justify-center" />
        </div>
        <hr className="mx-4 mt-10 max-w-measure border-line md:mx-auto" />
      </header>
    );
  }

  return (
    <header className="mx-auto box-content max-w-wide px-4">
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-1 border-b border-line py-4.5">
        <Link to="/" className="py-2 text-2xl leading-loose font-bold tracking-wide">
          {settings.title}
        </Link>
        <Nav items={items} />
      </div>
    </header>
  );
}

function Nav({ items, className }: { items: NavItem[]; className?: string }) {
  const { pathname } = useLocation();
  return (
    <nav aria-label="主导航" className={cn("flex flex-wrap gap-x-6 text-base leading-loose", className)}>
      {items.map((item) => {
        if (!item.url.startsWith("/")) {
          return (
            <a key={item.url} href={item.url} className={navLink}>
              {item.label}
            </a>
          );
        }
        const active = item.match ? item.match(pathname) : pathname === item.url || pathname.startsWith(`${item.url}/`);
        return (
          <Link key={item.url} to={item.url} className={navLink} aria-current={active ? "page" : undefined}>
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}

const navLink =
  "inline-block border-b border-transparent pt-2.5 pb-0.5 aria-[current=page]:border-accent aria-[current=page]:text-accent";
