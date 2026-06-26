import { EditorRoute } from "~/app/admin/editor/EditorRoute";

export function meta() {
  return [{ title: "写作 · 后台" }];
}

export default function PostEditor() {
  return <EditorRoute kind="post" />;
}
