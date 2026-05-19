// SkipLink 是键盘用户的「跳到正文」入口，平时藏在视口外，获得焦点时滑入。
export function SkipLink({ href, children }: { href: string; children: string }) {
  return (
    <a
      href={href}
      className="absolute -top-25 left-4 z-10 rounded-sm bg-accent px-3.5 py-2 text-paper hover:text-paper focus:top-3"
    >
      {children}
    </a>
  );
}
