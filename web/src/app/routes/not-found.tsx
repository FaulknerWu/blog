import { data } from "react-router";

// 多级路径不会命中任何页面，交给根错误边界渲染 404。
export function loader() {
  throw data(null, { status: 404 });
}

export default function NotFound() {
  return null;
}
