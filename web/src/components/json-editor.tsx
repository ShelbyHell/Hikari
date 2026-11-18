import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { json } from "@codemirror/lang-json";
import { HighlightStyle, indentOnInput, syntaxHighlighting } from "@codemirror/language";
import { tags } from "@lezer/highlight";
import { EditorState } from "@codemirror/state";
import { drawSelection, EditorView, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from "@codemirror/view";
import { useEffect, useRef } from "react";

const highlight = HighlightStyle.define([
  { tag: [tags.propertyName, tags.definition(tags.propertyName)], color: "var(--mikan-700)" },
  { tag: [tags.string, tags.special(tags.string)], color: "var(--leaf-700)" },
  { tag: [tags.number, tags.bool, tags.null, tags.atom], color: "var(--lagoon-600)" },
  { tag: [tags.keyword, tags.typeName, tags.labelName], color: "var(--fc-purple)" },
  { tag: tags.comment, color: "var(--ink-400)", fontStyle: "italic" },
  { tag: [tags.punctuation, tags.separator, tags.bracket, tags.operator], color: "var(--ink-500)" },
  { tag: tags.invalid, color: "var(--berry-600)" },
]);

const theme = EditorView.theme({
  "&": { fontSize: "13px", backgroundColor: "transparent" },
  ".cm-content": { fontFamily: "var(--font-mono-stack)", padding: "12px 0" },
  ".cm-content, .cm-line": { color: "var(--ink-900)", caretColor: "var(--ink-900)" },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--ink-900)" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": { backgroundColor: "color-mix(in srgb, var(--mikan-500) 24%, transparent)" },
  ".cm-gutters": { backgroundColor: "transparent", border: "none", color: "var(--ink-400)", paddingLeft: "4px" },
  ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "var(--hover)" },
  "&.cm-focused": { outline: "none" },
});

export function JsonEditor({
  value,
  onChange,
  label,
  invalid,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  label: string;
  invalid?: boolean;
  className?: string;
}) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const change = useRef(onChange);
  change.current = onChange;

  useEffect(() => {
    const v = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          history(),
          drawSelection(),
          indentOnInput(),
          syntaxHighlighting(highlight),
          keymap.of([...defaultKeymap, ...historyKeymap]),
          json(),
          theme,
          EditorView.lineWrapping,
          EditorView.contentAttributes.of({ "aria-label": label, spellcheck: "false" }),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) change.current(u.state.doc.toString());
          }),
        ],
      }),
    });
    view.current = v;
    return () => v.destroy();
  }, []);

  useEffect(() => {
    const v = view.current;
    if (v && v.state.doc.toString() !== value) {
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: value } });
    }
  }, [value]);

  return <div ref={host} className={`config-editor ${className ?? ""}`.trim()} aria-invalid={invalid || undefined} />;
}
