import { EditorRoute } from "~/app/admin/editor/EditorRoute";

export function meta() {
  return [{ title: "编辑页面 · 后台" }];
}

export default function PageEditor() {
  return <EditorRoute kind="page" />;
}
