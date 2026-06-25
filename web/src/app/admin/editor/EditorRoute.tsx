import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { data, useLocation, useNavigate, useParams } from "react-router";
import { EditorScreen } from "./EditorScreen";
import { load, type Kind } from "./model";

// EditorRoute 处理 /admin/{posts,pages}/:id；:id 为 new 时新建。
// 新建内容首次保存后替换地址，并在 state 里带上原实例的 key，让同一个编辑器继续工作而不重新加载。
export function EditorRoute({ kind }: { kind: Kind }) {
  const { id = "new" } = useParams();
  const location = useLocation();
  const key = (location.state as { editorKey?: string } | null)?.editorKey ?? `${id}:${location.key}`;
  return <Loader key={key} instance={key} kind={kind} param={id} />;
}

function Loader({ kind, param, instance }: { kind: Kind; param: string; instance: string }) {
  const navigate = useNavigate();
  // 只按首次进入时的地址加载，新建后地址变化不触发重新加载。
  const [initialParam] = useState(param);
  const id = initialParam === "new" ? null : Number(initialParam);
  if (id !== null && !(Number.isInteger(id) && id > 0)) throw data(null, { status: 404 });

  const doc = useQuery({
    queryKey: [kind, "editor", id],
    queryFn: () => load(kind, id!),
    enabled: id !== null,
    staleTime: Infinity,
    gcTime: 0,
  });

  const onCreated = (newId: number) =>
    navigate(`/admin/${kind === "post" ? "posts" : "pages"}/${newId}`, { replace: true, state: { editorKey: instance } });

  if (id === null) return <EditorScreen kind={kind} initial={null} onCreated={onCreated} />;
  if (doc.isPending) return <div className="min-h-screen bg-paper" aria-busy="true" />;
  if (doc.isError) throw doc.error;
  return <EditorScreen kind={kind} initial={doc.data} onCreated={onCreated} />;
}
