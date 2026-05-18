import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

// 告诉 tailwind-merge 主题里自定义的取值名（字号 md、版心 measure/wide、阴影 dialog 等），
// 否则它认不出 max-w-wide 与 max-w-measure 是同一类，后者无法覆盖前者。
// v4 的 leading-* 通过 --tw-leading 与字号自带的行高组合，两者不冲突，不让字号类吞掉行高类。
const twMerge = extendTailwindMerge({
  override: {
    conflictingClassGroups: { "font-size": [] },
  },
  extend: {
    theme: {
      text: ["xs", "sm", "md", "base", "lg", "xl", "2xl", "3xl", "4xl", "5xl", "6xl", "7xl"],
      container: ["measure", "wide"],
      shadow: ["popover", "dialog"],
      font: ["math"],
    },
  },
});

// cn 拼接类名，并让后传入的工具类覆盖前面冲突的同类工具类。
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
